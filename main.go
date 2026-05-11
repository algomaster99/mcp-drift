package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
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

const defaultRegistryURL = "https://registry.modelcontextprotocol.io/v0/servers"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// headerFlag is a repeatable -header flag that accumulates Name:Value pairs.
type headerFlag []string

func (h *headerFlag) String() string { return strings.Join(*h, ", ") }
func (h *headerFlag) Set(v string) error {
	if !strings.Contains(v, ":") {
		return fmt.Errorf("header must be in Name:Value format")
	}
	*h = append(*h, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift <initialize|tools-list|registry-list> [flags] [url]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "initialize":
		runInitialize(os.Args[2:])
	case "tools-list":
		runToolsList(os.Args[2:])
	case "registry-list":
		runRegistryList(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func runInitialize(args []string) {
	fs := flag.NewFlagSet("initialize", flag.ExitOnError)
	var headers headerFlag
	fs.Var(&headers, "header", "extra request header in Name:Value format (repeatable)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift initialize [--header Name:Value] <url>")
		os.Exit(2)
	}

	req, err := http.NewRequest("POST", fs.Arg(0), strings.NewReader(initBody))
	if err != nil {
		fatal("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyHeaders(req, headers)

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
	session := fs.String("session", "", "mcp-session-id from initialize")
	var headers headerFlag
	fs.Var(&headers, "header", "extra request header in Name:Value format (repeatable)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift tools-list [--session ID] [--header Name:Value] <url>")
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
	applyHeaders(req, headers)

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

// registryServer is the normalized form we store in snapshots.
type registryServer struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Version     string           `json:"version"`
	Description string           `json:"description,omitempty"`
	Remotes     []registryRemote `json:"remotes,omitempty"`
	Repository  *registryRepo    `json:"repository,omitempty"`
	WebsiteURL  string           `json:"websiteUrl,omitempty"`
}

type registryRemote struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type registryRepo struct {
	URL    string `json:"url,omitempty"`
	Source string `json:"source,omitempty"`
}

func runRegistryList(args []string) {
	fs := flag.NewFlagSet("registry-list", flag.ExitOnError)
	registryBase := fs.String("url", defaultRegistryURL, "registry API base URL")
	transport := fs.String("transport", "", "filter by transport type (e.g. streamable-http); empty means all")
	fs.Parse(args)

	var servers []registryServer
	cursor := ""

	for {
		apiURL := fmt.Sprintf("%s?limit=100", *registryBase)
		if cursor != "" {
			apiURL += "&cursor=" + url.QueryEscape(cursor)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			fatal("build registry request: %v", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			fatal("fetch registry: %v", err)
		}

		var page struct {
			Servers []struct {
				Server struct {
					Name        string           `json:"name"`
					Title       string           `json:"title"`
					Version     string           `json:"version"`
					Description string           `json:"description"`
					Remotes     []registryRemote `json:"remotes"`
					Repository  *registryRepo    `json:"repository"`
					WebsiteURL  string           `json:"websiteUrl"`
				} `json:"server"`
				Meta struct {
					Official struct {
						IsLatest bool `json:"isLatest"`
					} `json:"io.modelcontextprotocol.registry/official"`
				} `json:"_meta"`
			} `json:"servers"`
			Metadata struct {
				NextCursor string `json:"nextCursor"`
			} `json:"metadata"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			fatal("decode registry page: %v", err)
		}
		resp.Body.Close()

		for _, entry := range page.Servers {
			if !entry.Meta.Official.IsLatest {
				continue
			}
			s := entry.Server

			// Apply transport filter if requested.
			if *transport != "" {
				matched := false
				for _, r := range s.Remotes {
					if r.Type == *transport {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			srv := registryServer{
				Name:        s.Name,
				Title:       s.Title,
				Version:     s.Version,
				Description: s.Description,
				Remotes:     s.Remotes,
				Repository:  s.Repository,
				WebsiteURL:  s.WebsiteURL,
			}
			servers = append(servers, srv)
		}

		if page.Metadata.NextCursor == "" {
			break
		}
		cursor = page.Metadata.NextCursor
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	out, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		fatal("marshal output: %v", err)
	}
	fmt.Println(string(out))
}

func applyHeaders(req *http.Request, headers headerFlag) {
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(2)
}
