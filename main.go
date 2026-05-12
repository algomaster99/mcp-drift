package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const initBody = `{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "capabilities": {},
    "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
  }
}`

const toolsBody = `{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "tools/list",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
  }
}`

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift <initialize|tools-list> [flags] <url>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "initialize":
		runInitialize(os.Args[2:])
	case "tools-list":
		runToolsList(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func runInitialize(args []string) {
	fs := flag.NewFlagSet("initialize", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift initialize <url>")
		os.Exit(2)
	}

	req, err := http.NewRequest("POST", fs.Arg(0), strings.NewReader(initBody))
	if err != nil {
		fatal("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fatal("initialize: %v", err)
	}
	resp.Body.Close()

	session := resp.Header.Get("mcp-session-id")
	if session == "" {
		fatal("no mcp-session-id in response headers (status %d)", resp.StatusCode)
	}
	fmt.Print(session)
}

func runToolsList(args []string) {
	fs := flag.NewFlagSet("tools-list", flag.ExitOnError)
	session := fs.String("session", "", "mcp-session-id from initialize (omit for stateless servers)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift tools-list [--session ID] <url>")
		os.Exit(2)
	}

	req, err := http.NewRequest("POST", fs.Arg(0), strings.NewReader(toolsBody))
	if err != nil {
		fatal("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if *session != "" {
		req.Header.Set("mcp-session-id", *session)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		fatal("tools/list: %v", err)
	}
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		fatal("decode response: %v", err)
	}

	if errField, ok := raw["error"]; ok && string(errField) != "null" {
		fatal("rpc error: %s", errField)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw["result"], &result); err != nil {
		fatal("decode result: %v", err)
	}

	var tools []json.RawMessage
	if err := json.Unmarshal(result["tools"], &tools); err != nil {
		fatal("decode tools: %v", err)
	}

	out, _ := json.MarshalIndent(tools, "", "  ")
	fmt.Println(string(out))
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(2)
}
