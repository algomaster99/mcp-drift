package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	protocolVersion = "2025-11-25"
	clientName      = "mcp-drift"
	clientVersion   = "0.1.0"
)

type Client struct {
	httpClient *http.Client
}

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ServerCapabilities reflects which MCP list methods the server advertises.
type ServerCapabilities struct {
	Session   string `json:"session"`
	Tools     bool   `json:"tools"`
	Prompts   bool   `json:"prompts"`
	Resources bool   `json:"resources"`
}

type listResult struct {
	Items      []json.RawMessage
	NextCursor string
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

// Initialize calls the MCP initialize method and returns the server's advertised
// capabilities. Session is empty for stateless servers (no mcp-session-id header).
func (c *Client) Initialize(ctx context.Context, url string) (*ServerCapabilities, error) {
	body := rpcRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    clientName,
				"version": clientVersion,
			},
		},
	}

	resp, err := c.post(ctx, url, "", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, bytes.TrimSpace(responseBody))
	}

	var raw rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode initialize response: %w", err)
	}
	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		return nil, decodeRPCError(raw.Error)
	}

	// Parse capabilities from the result body.
	var result struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(raw.Result, &result); err != nil {
		return nil, fmt.Errorf("decode initialize result: %w", err)
	}

	caps := &ServerCapabilities{
		Session:   resp.Header.Get("mcp-session-id"),
		Tools:     result.Capabilities["tools"] != nil,
		Prompts:   result.Capabilities["prompts"] != nil,
		Resources: result.Capabilities["resources"] != nil,
	}
	return caps, nil
}

func (c *Client) ListAll(ctx context.Context, url, session, method, resultKey string) ([]json.RawMessage, error) {
	all := []json.RawMessage{}
	var cursor string

	for page := 1; ; page++ {
		result, err := c.listPage(ctx, url, session, method, resultKey, cursor, page)
		if err != nil {
			return nil, err
		}

		all = append(all, result.Items...)
		if result.NextCursor == "" {
			return all, nil
		}
		cursor = result.NextCursor
	}
}

func (c *Client) listPage(ctx context.Context, url, session, method, resultKey, cursor string, page int) (listResult, error) {
	params := map[string]any{}
	if cursor != "" {
		params["cursor"] = cursor
	}

	body := rpcRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("%d", page),
		Method:  method,
		Params:  params,
	}

	resp, err := c.post(ctx, url, session, body)
	if err != nil {
		return listResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return listResult{}, fmt.Errorf("http status %d: %s", resp.StatusCode, bytes.TrimSpace(responseBody))
	}

	var raw rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return listResult{}, fmt.Errorf("decode response: %w", err)
	}
	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		return listResult{}, decodeRPCError(raw.Error)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw.Result, &result); err != nil {
		return listResult{}, fmt.Errorf("decode result: %w", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(result[resultKey], &items); err != nil {
		return listResult{}, fmt.Errorf("decode %s: %w", resultKey, err)
	}
	if items == nil {
		items = []json.RawMessage{}
	}

	var nextCursor string
	if rawCursor, ok := result["nextCursor"]; ok {
		if err := json.Unmarshal(rawCursor, &nextCursor); err != nil {
			return listResult{}, fmt.Errorf("decode nextCursor: %w", err)
		}
	}

	return listResult{Items: items, NextCursor: nextCursor}, nil
}

func (c *Client) post(ctx context.Context, url, session string, body rpcRequest) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		req.Header.Set("mcp-session-id", session)
	}

	return c.httpClient.Do(req)
}

func (e *RPCError) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("rpc error %d: %s: %s", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

func IsMethodUnsupported(err error) bool {
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		return false
	}

	message := strings.ToLower(rpcErr.Message)
	return rpcErr.Code == -32601 || strings.Contains(message, "method not found") || strings.Contains(message, "method not supported")
}

func decodeRPCError(raw json.RawMessage) error {
	var rpcErr RPCError
	if err := json.Unmarshal(raw, &rpcErr); err != nil || rpcErr.Message == "" {
		return fmt.Errorf("rpc error: %s", raw)
	}
	return &rpcErr
}
