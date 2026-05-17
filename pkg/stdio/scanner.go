package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"mcp-drift/pkg/strace"
)

const (
	protocolVersion = "2025-11-25"
	clientName      = "mcp-drift"
	clientVersion   = "0.1.0"
)

type ScanResult struct {
	Tools        []json.RawMessage `json:"tools"`
	Prompts      []json.RawMessage `json:"prompts"`
	Resources    []json.RawMessage `json:"resources"`
	Network      []strace.NetCall  `json:"network"`
	Subprocesses []strace.ExecCall `json:"subprocesses"`
}

type mcpClient struct {
	scanner *bufio.Scanner
	writer  io.Writer
	nextID  int
}

// Scan runs cmdArgs under strace (if available), speaks MCP over stdio, and
// returns tools/prompts/resources lists alongside observed network and subprocess calls.
// If saveStraceTo is non-empty, the raw strace log is copied there before deletion.
func Scan(ctx context.Context, cmdArgs []string, saveStraceTo string) (*ScanResult, error) {
	straceLog, err := os.CreateTemp("", "mcp-strace-*.log")
	if err != nil {
		return nil, fmt.Errorf("create strace log: %w", err)
	}
	logPath := straceLog.Name()
	straceLog.Close()
	defer func() {
		if saveStraceTo != "" {
			os.Rename(logPath, saveStraceTo)
		} else {
			os.Remove(logPath)
		}
	}()

	useStrace := hasStrace()

	var cmd *exec.Cmd
	if useStrace {
		args := append([]string{"-f", "-s", "256", "-e", "trace=connect,execve", "-o", logPath}, cmdArgs...)
		cmd = exec.CommandContext(ctx, "strace", args...)
	} else {
		cmd = exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %q: %w", cmdArgs[0], err)
	}

	c := &mcpClient{
		scanner: bufio.NewScanner(stdoutPipe),
		writer:  stdinPipe,
	}
	c.scanner.Buffer(make([]byte, 4<<20), 4<<20)

	result, scanErr := c.run(ctx)
	stdinPipe.Close()

	// SIGINT propagates to the traced child and lets strace flush its -o log.
	// Fall back to SIGKILL if the process hasn't exited within 2 seconds.
	cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Minute):
		cmd.Process.Kill()
		<-done
	}

	if useStrace {
		f, err := os.Open(logPath)
		if err == nil {
			calls := strace.Parse(f)
			f.Close()
			if result != nil {
				result.Network = calls.Network
				result.Subprocesses = calls.Subprocesses
			}
		}
	}

	return result, scanErr
}

func hasStrace() bool {
	_, err := exec.LookPath("strace")
	return err == nil
}

func (c *mcpClient) send(method string, params map[string]any) (string, error) {
	c.nextID++
	id := fmt.Sprintf("%d", c.nextID)
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if len(params) > 0 {
		msg["params"] = params
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	_, err = c.writer.Write(b)
	return id, err
}

func (c *mcpClient) notify(method string) {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	b, _ := json.Marshal(msg)
	c.writer.Write(append(b, '\n'))
}

// recv reads lines until it finds a response matching id, skipping notifications.
func (c *mcpClient) recv(id string) (json.RawMessage, error) {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		var resp struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		if resp.Method != "" {
			continue // notification
		}
		var respID string
		json.Unmarshal(resp.ID, &respID)
		if respID != id {
			continue
		}
		if len(resp.Error) > 0 && string(resp.Error) != "null" {
			return nil, fmt.Errorf("rpc error: %s", resp.Error)
		}
		return resp.Result, nil
	}
	return nil, io.EOF
}

func (c *mcpClient) listAll(method, resultKey string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		id, err := c.send(method, params)
		if err != nil {
			return nil, err
		}
		result, err := c.recv(id)
		if err != nil {
			return nil, err
		}
		var r map[string]json.RawMessage
		if err := json.Unmarshal(result, &r); err != nil {
			return nil, err
		}
		var items []json.RawMessage
		json.Unmarshal(r[resultKey], &items)
		all = append(all, items...)
		var nextCursor string
		if cur, ok := r["nextCursor"]; ok {
			json.Unmarshal(cur, &nextCursor)
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	if all == nil {
		all = []json.RawMessage{}
	}
	return all, nil
}

func (c *mcpClient) run(ctx context.Context) (*ScanResult, error) {
	id, err := c.send("initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": clientName, "version": clientVersion},
	})
	if err != nil {
		return nil, fmt.Errorf("send initialize: %w", err)
	}

	initResult, err := c.recv(id)
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	_ = initResult // capabilities field intentionally ignored — we probe dynamically below

	c.notify("notifications/initialized")

	result := &ScanResult{
		Tools:        []json.RawMessage{},
		Prompts:      []json.RawMessage{},
		Resources:    []json.RawMessage{},
		Network:      []strace.NetCall{},
		Subprocesses: []strace.ExecCall{},
	}

	// Probe all three list methods regardless of what initialize advertised.
	// Servers sometimes mis-advertise capabilities; probing catches both gaps and
	// undeclared methods that still work.
	if tools, err := c.listAll("tools/list", "tools"); err == nil {
		result.Tools = tools
	} else if !isUnsupported(err) {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	if prompts, err := c.listAll("prompts/list", "prompts"); err == nil {
		result.Prompts = prompts
	} else if !isUnsupported(err) {
		return nil, fmt.Errorf("prompts/list: %w", err)
	}

	if resources, err := c.listAll("resources/list", "resources"); err == nil {
		result.Resources = resources
	} else if !isUnsupported(err) {
		return nil, fmt.Errorf("resources/list: %w", err)
	}

	return result, nil
}

// isUnsupported returns true for JSON-RPC -32601 (method not found/supported).
func isUnsupported(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "-32601") ||
		strings.Contains(strings.ToLower(s), "method not found") ||
		strings.Contains(strings.ToLower(s), "method not supported")
}
