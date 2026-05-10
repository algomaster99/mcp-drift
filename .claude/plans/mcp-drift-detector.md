# Plan: MCP Drift Detector

## Context

Public MCP server URLs (e.g. `https://huggingface.co/mcp`) are unversioned. A server
operator can silently change the tool list at any time — adding a malicious tool that
exfiltrates tokens, altering tool descriptions to hijack LLM behavior, etc.

The MCP spec (2025-11-25) does define a `notifications/tools/list_changed` notification, but it is unclear how to receive it.
When does the server send it? I do not see a mechanism that it tracks particular clients.

Thus, we use polling to detect drift.

---

## Approach

A minimal Go binary with two subcommands handles the MCP protocol; each server gets one
GitHub Actions workflow file that calls it.

```
mcp-drift initialize --token TOKEN <url>   → prints mcp-session-id to stdout
mcp-drift tools-list --token TOKEN --session ID <url>  → prints canonical JSON to stdout
```

The workflow becomes two clean named steps: one calls `initialize`, one calls
`tools-list`. No curl/grep/awk in YAML. The binary is stdlib-only (no dependencies).

---

## Directory Structure

```
mcp-drift/
├── main.go                      # Go binary — initialize + tools-list subcommands
├── go.mod
├── .github/
│   └── workflows/
│       └── hf.yml               # HuggingFace drift detection (one file per server)
├── snapshots/
│   ├── tools/
│   │   └── huggingface.json     # tools/list snapshot
│   ├── prompts/                 # future: prompts/list snapshots
│   └── resources/               # future: resources/list snapshots
└── README.md
```

Adding a new server = add a new workflow file + bootstrap its snapshot.

---

## MCP Protocol Flow (HuggingFace)

Two POST requests to `https://huggingface.co/mcp`:

**Step 1 — initialize** — captures `mcp-session-id` from response headers:

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "capabilities": {},
    "clientInfo": {
      "name": "mcp-drift",
      "version": "0.1.0"
    }
  }
}
```

Response headers contain `mcp-session-id` — required for all subsequent requests.

**Step 2 — tools/list** — sent with `mcp-session-id` header:

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "tools/list",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "mcp-drift",
      "version": "0.1.0"
    }
  }
}
```

`jq` extracts only the `tools` array, picks stable fields, and sorts by `name` —
producing canonical output that avoids false-positive diffs from ordering changes.

---

## Snapshot Format — `snapshots/huggingface.json`

Just tools array, pretty-printed:

```json
[
  {
    "description": "...",
    "inputSchema": { ... },
    "name": "tool_name"
  }
]
```

---

## Workflow — `.github/workflows/hf.yml`

```yaml
name: HuggingFace MCP Drift

on:
  schedule:
    - cron: '0 8 * * *'
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Initialize — get mcp-session-id
        env:
          HF_TOKEN: ${{ secrets.HF_TOKEN }}
        run: |
          SESSION=$(curl -s -D - -X POST https://huggingface.co/mcp \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $HF_TOKEN" \
            -d '{
              "jsonrpc": "2.0",
              "id": "1",
              "method": "initialize",
              "params": {
                "protocolVersion": "2025-03-26",
                "capabilities": {},
                "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
              }
            }' \
            | grep -i 'mcp-session-id' | awk '{print $2}' | tr -d '\r')
          echo "MCP_SESSION=$SESSION" >> $GITHUB_ENV

      - name: Fetch tools/list
        env:
          HF_TOKEN: ${{ secrets.HF_TOKEN }}
        run: |
          curl -s -X POST https://huggingface.co/mcp \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $HF_TOKEN" \
            -H "mcp-session-id: $MCP_SESSION" \
            -d '{
              "jsonrpc": "2.0",
              "id": "1",
              "method": "tools/list",
              "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
              }
            }' \
            | jq '[.result.tools[] | {name,description,inputSchema}] | sort_by(.name)' \
            > live.json

      - name: Compare to snapshot
        id: diff
        run: |
          if diff -q snapshots/huggingface.json live.json > /dev/null 2>&1; then
            echo "drifted=false" >> $GITHUB_OUTPUT
          else
            echo "drifted=true" >> $GITHUB_OUTPUT
            DIFF=$(diff snapshots/huggingface.json live.json || true)
            {
              echo 'diff<<EOF'
              echo "$DIFF"
              echo 'EOF'
            } >> $GITHUB_OUTPUT
          fi

      - name: Open PR if drifted
        if: steps.diff.outputs.drifted == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          cp live.json snapshots/huggingface.json
          BRANCH="drift/huggingface-$(date +%Y-%m-%d)"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git config user.name "github-actions[bot]"
          git checkout -b "$BRANCH"
          git add snapshots/huggingface.json
          git commit -m "drift: huggingface tools/list changed $(date +%Y-%m-%d)"
          git push origin "$BRANCH"
          gh pr create \
            --title "MCP drift detected: huggingface ($(date +%Y-%m-%d))" \
            --body "$(printf '## Tool list changed\n\n```diff\n%s\n```' "${{ steps.diff.outputs.diff }}")" \
            --base main \
            --head "$BRANCH"
```

---

## Bootstrap (First-Time Setup)

```bash
# Step 1: initialize — get mcp-session-id
SESSION=$(curl -s -D - -X POST https://huggingface.co/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HF_TOKEN" \
  -d '{
    "jsonrpc": "2.0", "id": "1", "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26", "capabilities": {},
      "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
    }
  }' \
  | grep -i 'mcp-session-id' | awk '{print $2}' | tr -d '\r')

# Step 2: tools/list — write initial snapshot
curl -s -X POST https://huggingface.co/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HF_TOKEN" \
  -H "mcp-session-id: $SESSION" \
  -d '{
    "jsonrpc": "2.0", "id": "1", "method": "tools/list",
    "params": {
      "protocolVersion": "2024-11-05", "capabilities": {},
      "clientInfo": { "name": "mcp-drift", "version": "0.1.0" }
    }
  }' \
  | jq '[.result.tools[] | {name,description,inputSchema}] | sort_by(.name)' \
  > snapshots/huggingface.json

# 2. Commit
git add snapshots/huggingface.json .github/workflows/hf.yml
git commit -m "add huggingface MCP snapshot"
```

---

## Verification

```bash
# Locally: simulate what CI does
diff snapshots/huggingface.json live.json   # should be empty after bootstrap

# Simulate drift: edit a tool name in the snapshot, re-fetch, re-diff
# CI: trigger workflow_dispatch in GitHub Actions UI
```