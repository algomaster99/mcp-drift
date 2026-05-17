package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"mcp-drift/pkg/lists"
	"mcp-drift/pkg/mcp"
	"mcp-drift/pkg/stdio"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift <initialize|tools-list|resources-list|prompts-list|stdio-scan> [flags] <url|command>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "initialize":
		runInitialize(os.Args[2:])
	case "tools-list":
		runList("tools-list", lists.Tools, os.Args[2:])
	case "resources-list":
		runList("resources-list", lists.Resources, os.Args[2:])
	case "prompts-list":
		runList("prompts-list", lists.Prompts, os.Args[2:])
	case "stdio-scan":
		runStdioScan(os.Args[2:])
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

	caps, err := mcp.NewClient().Initialize(context.Background(), fs.Arg(0))
	if err != nil {
		fatal("initialize: %v", err)
	}

	out, _ := json.Marshal(caps)
	fmt.Println(string(out))
}

func runList(name string, list lists.List, args []string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	session := fs.String("session", "", "mcp-session-id from initialize (omit for stateless servers)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: mcp-drift %s [--session ID] <url>\n", name)
		os.Exit(2)
	}

	items, err := list.Fetch(context.Background(), mcp.NewClient(), fs.Arg(0), *session)
	if err != nil {
		if list.AllowUnsupported && mcp.IsMethodUnsupported(err) {
			items = []json.RawMessage{}
		} else {
			fatal("%s: %v", list.Method, err)
		}
	}

	out, _ := json.MarshalIndent(items, "", "  ")
	fmt.Println(string(out))
}

func runStdioScan(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: mcp-drift stdio-scan <command> [args...]")
		os.Exit(2)
	}

	result, err := stdio.Scan(context.Background(), args)
	if err != nil {
		fatal("stdio-scan: %v", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(2)
}
