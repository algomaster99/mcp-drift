# mcp-drift

**mcp-drift** detects silent changes by polling public MCP servers daily and diffing
against a committed snapshot. If the tool list changes, it opens a pull request with the
diff so the change is on record and reviewable.

## Why?

MCP server URLs are not versioned. A server operator can silently change the tool list
at any time by adding a malicious tool that exfiltrates tokens, altering descriptions to
hijack LLM behaviour, or removing tools without notice.

The MCP spec defines a [`notifications/tools/list_changed`](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#capabilities)
notification, but it is unclear at what stage this notification is dispatched.
A client cannot indefinitely have a persistent connection with the MCP server.

## Monitored Servers

| Server | Status |
| ------ | ------ |
| [huggingface.co/mcp] | [![][huggingface-badge]][huggingface-url] |

[huggingface.co/mcp]: https://huggingface.co/mcp
[huggingface-badge]: https://github.com/algomaster99/mcp-drift/actions/workflows/huggingface.yml/badge.svg?event=schedule
[huggingface-url]: https://github.com/algomaster99/mcp-drift/actions/workflows/huggingface.yml

## How It Works

1. A GitHub Actions workflow runs daily for each server.
2. It calls the MCP `initialize` endpoint to get a session, then fetches `tools/list`.
3. The result is diffed against `snapshots/tools/<server>.json`.
4. If anything changed, a PR is opened with the diff in the description.

The first snapshot for each server is bootstrapped locally:

```sh
go build -o mcp-drift .
SESSION=$(./mcp-drift initialize https://huggingface.co/mcp)
./mcp-drift tools-list --session "$SESSION" https://huggingface.co/mcp \
  > snapshots/tools/huggingface.json
```