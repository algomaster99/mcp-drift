# mcp-drift

**mcp-drift** detects silent changes by polling public MCP servers daily and diffing
against committed snapshots. If an MCP list changes, it opens a pull request with the
diff so the change is on record and reviewable.

## Why?

MCP server URLs are not versioned. A server operator can silently change the tool list
at any time by adding a malicious tool that exfiltrates tokens, altering descriptions to
hijack LLM behaviour, or removing tools without notice.

The MCP spec defines a [`notifications/tools/list_changed`](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#capabilities)
notification, but it is unclear at what stage this notification is dispatched.
A client cannot indefinitely have a persistent connection with the MCP server.

## Monitored Servers

| Server | `tools/list` | `prompts/list` | `resources/list` |
| ------ | ------------ | -------------- | ---------------- |
| [huggingface.co/mcp] | 7 | 0 | ✗ |
| [developerknowledge.googleapis.com/mcp] | 2 | 1 | ✗ |
| [gmailmcp.googleapis.com/mcp/v1] | 2 | 0 | ✗ |
| [drivemcp.googleapis.com/mcp/v1] | 1 | 0 | ✗ |
| [calendarmcp.googleapis.com/mcp/v1] | 3 | 0 | ✗ |
| [mcp.canva.com/mcp] | ? | ? | ? |
| [mcp.na1.ironcladapp.com/mcp] | ? | ? | ? |
| [microsoft365.mcp.claude.com/mcp] | ? | ? | ? |
| [mcp.figma.com/mcp] | ? | ? | ? |
| [mcp.notion.com/mcp] | ? | ? | ? |
| [mcp.atlassian.com/v1/mcp] | ? | ? | ? |
| [web-api.us.netdocuments.app/connect/mcp] | ? | ? | ? |
| [legal-mcp.thomsonreuters.com/mcp] | ? | ? | ? |
| [mcp.zoom.us/mcp/zoom/streamable] | ? | ? | ? |
| [mcp.slack.com/mcp] | ? | ? | ? |
| [mcp.ramp.com/ramp-data/anthropic/mcp] | 1 | ✗ | ✗ |
| [integrations.getmontecarlo.com/mcp] | ? | ? | ? |
| [mcp.hubspot.com/anthropic] | ? | ? | ? |
| [mcp.courtlistener.com] | ? | ? | ? |
| [mcp.courtroom5.com/v1] | ? | ? | ? |
| [www.expedia.com/mcp] | ? | ? | ? |
| [mcp.linear.app/mcp] | ? | ? | ? |
| [mcp.monday.com/mcp] | ? | ? | ? |
| [mcp.descrybe.com/mcp] | 2 | ✗ | ✗ |
| [mcp.asana.com/v2/mcp] | ? | ? | ? |
| [mcp.miro.com] | ? | ? | ? |
| [api.solveintelligence.com/mcp] | ? | ? | ? |
| [mcp.twilio.com/docs] | 1 | ✗ | ✗ |
| [mcp.intercom.com/mcp] | ? | ? | ? |
| [mcp.mintlify.com] | ? | ? | ? |
| [agents.riskanalytics.dnb.com/mcp] | ? | ? | ? |
| [api.ziprecruiter.com/mcp] | 3 | ✗ | 1 |
| [mcp.datadoghq.com/api/unstable/mcp-server/mcp] | ? | ? | ? |
| [mcp.box.com] | ? | ? | ? |
| [mcp.granola.ai/mcp] | ? | ? | ? |
| [mcp.lucid.app/mcp] | ? | ? | ? |
| [setup.shopify.com/mcp] | ? | ? | ? |
| [learn.microsoft.com/api/mcp] | ? | ? | ? |
| [uakozrqrztgrgwoywxkx.supabase.co/functions/v1/mcp-server] | ? | ✗ | ✗ |
| [adobe-creativity.adobe.io/mcp] | ? | ? | ? |
| [mcp.privacy.com] | ? | ? | ? |
| [mcp.pandadoc.com/v1/mcp] | ? | ? | ? |
| [api.everlaw.com/v1/mcp] | ? | ? | ? |
| [api.techgc.co/api/mcp/topcounsel] | ? | ? | ? |

[huggingface.co/mcp]: https://huggingface.co/mcp
[developerknowledge.googleapis.com/mcp]: https://developerknowledge.googleapis.com/mcp
[gmailmcp.googleapis.com/mcp/v1]: https://gmailmcp.googleapis.com/mcp/v1
[drivemcp.googleapis.com/mcp/v1]: https://drivemcp.googleapis.com/mcp/v1
[calendarmcp.googleapis.com/mcp/v1]: https://calendarmcp.googleapis.com/mcp/v1
[mcp.canva.com/mcp]: https://mcp.canva.com/mcp
[mcp.na1.ironcladapp.com/mcp]: https://mcp.na1.ironcladapp.com/mcp
[microsoft365.mcp.claude.com/mcp]: https://microsoft365.mcp.claude.com/mcp
[mcp.figma.com/mcp]: https://mcp.figma.com/mcp
[mcp.notion.com/mcp]: https://mcp.notion.com/mcp
[mcp.atlassian.com/v1/mcp]: https://mcp.atlassian.com/v1/mcp
[web-api.us.netdocuments.app/connect/mcp]: https://web-api.us.netdocuments.app/connect/mcp
[legal-mcp.thomsonreuters.com/mcp]: https://legal-mcp.thomsonreuters.com/mcp
[mcp.zoom.us/mcp/zoom/streamable]: https://mcp.zoom.us/mcp/zoom/streamable
[mcp.slack.com/mcp]: https://mcp.slack.com/mcp
[mcp.ramp.com/ramp-data/anthropic/mcp]: https://mcp.ramp.com/ramp-data/anthropic/mcp
[integrations.getmontecarlo.com/mcp]: https://integrations.getmontecarlo.com/mcp
[mcp.hubspot.com/anthropic]: https://mcp.hubspot.com/anthropic
[mcp.courtlistener.com]: https://mcp.courtlistener.com/
[mcp.courtroom5.com/v1]: https://mcp.courtroom5.com/v1
[www.expedia.com/mcp]: https://www.expedia.com/mcp
[mcp.linear.app/mcp]: https://mcp.linear.app/mcp
[mcp.monday.com/mcp]: https://mcp.monday.com/mcp
[mcp.descrybe.com/mcp]: https://mcp.descrybe.com/mcp
[mcp.asana.com/v2/mcp]: https://mcp.asana.com/v2/mcp
[mcp.miro.com]: https://mcp.miro.com/
[api.solveintelligence.com/mcp]: https://api.solveintelligence.com/mcp/
[mcp.twilio.com/docs]: https://mcp.twilio.com/docs
[mcp.intercom.com/mcp]: https://mcp.intercom.com/mcp
[mcp.mintlify.com]: https://mcp.mintlify.com
[agents.riskanalytics.dnb.com/mcp]: https://agents.riskanalytics.dnb.com/mcp
[api.ziprecruiter.com/mcp]: https://api.ziprecruiter.com/mcp
[mcp.datadoghq.com/api/unstable/mcp-server/mcp]: https://mcp.datadoghq.com/api/unstable/mcp-server/mcp?toolsets=all
[mcp.box.com]: https://mcp.box.com
[mcp.granola.ai/mcp]: https://mcp.granola.ai/mcp
[mcp.lucid.app/mcp]: https://mcp.lucid.app/mcp
[setup.shopify.com/mcp]: https://setup.shopify.com/mcp
[learn.microsoft.com/api/mcp]: https://learn.microsoft.com/api/mcp
[uakozrqrztgrgwoywxkx.supabase.co/functions/v1/mcp-server]: https://uakozrqrztgrgwoywxkx.supabase.co/functions/v1/mcp-server
[adobe-creativity.adobe.io/mcp]: https://adobe-creativity.adobe.io/mcp
[mcp.privacy.com]: https://mcp.privacy.com
[mcp.pandadoc.com/v1/mcp]: https://mcp.pandadoc.com/v1/mcp
[api.everlaw.com/v1/mcp]: https://api.everlaw.com/v1/mcp
[api.techgc.co/api/mcp/topcounsel]: https://api.techgc.co/api/mcp/topcounsel

The counts are the number of recorded drift updates for each snapshot file. Newly
monitored files start at zero.

- `✗` — server declared this capability absent in its `initialize` response; snapshot stores `false`
- `?` — server requires authentication; `initialize` returned 401 so capabilities are unknown; snapshot stores `null`

## How It Works

1. A GitHub Actions workflow runs daily for each server.
2. For stateful servers it calls `initialize` to get a session, then fetches `tools/list`,
   `prompts/list`, and `resources/list`. For stateless servers (e.g. Google APIs) it
   fetches the lists directly — no session needed.
3. The results are diffed against `snapshots/<list>/<server>.json`.
4. If anything changed, a PR is opened with the diff in the description.

The first snapshot for each server is bootstrapped locally.

Stateful server (HuggingFace):

```sh
go build -o mcp-drift .
SESSION=$(./mcp-drift initialize https://huggingface.co/mcp)
./mcp-drift tools-list --session "$SESSION" https://huggingface.co/mcp \
  > snapshots/tools/huggingface.json
./mcp-drift prompts-list --session "$SESSION" https://huggingface.co/mcp \
  > snapshots/prompts/huggingface.json
./mcp-drift resources-list --session "$SESSION" https://huggingface.co/mcp \
  > snapshots/resources/huggingface.json
```

Stateless server (Google Developer Knowledge):

```sh
go build -o mcp-drift .
./mcp-drift tools-list https://developerknowledge.googleapis.com/mcp \
  > snapshots/tools/google-developerknowledge.json
./mcp-drift prompts-list https://developerknowledge.googleapis.com/mcp \
  > snapshots/prompts/google-developerknowledge.json
./mcp-drift resources-list https://developerknowledge.googleapis.com/mcp \
  > snapshots/resources/google-developerknowledge.json
```

## Monitored stdio Servers

| Server | `tools/list` | `prompts/list` | `resources/list` | `network` | `subprocesses` |
| ------ | ------------ | -------------- | ---------------- | --------- | -------------- |
| Claude Code | 4 | 0 | 0 | 0 | 0 |

The counts are the number of recorded drift updates for each snapshot file. Newly
monitored files start at zero.

- `✗` — server declared this capability absent in its `initialize` response; snapshot stores `false`
- `network` and `subprocesses` are captured via `strace` and show the number of unique endpoints/processes observed at last scan

## Related Work

https://github.com/mcptrust/mcptrust: This implements a lockfile for MCP servers.
The lockfile consists of SHA256 hashes of the tool list.
The integrations enable the client to check the lockfile and abort if the hash has changed
(due to an unknown tool in MCP server).
