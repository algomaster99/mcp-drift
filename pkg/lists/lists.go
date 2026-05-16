package lists

import (
	"context"
	"encoding/json"

	"mcp-drift/pkg/mcp"
)

type List struct {
	Method    string
	ResultKey string
}

var (
	Tools     = List{Method: "tools/list", ResultKey: "tools"}
	Resources = List{Method: "resources/list", ResultKey: "resources"}
	Prompts   = List{Method: "prompts/list", ResultKey: "prompts"}
)

func (l List) Fetch(ctx context.Context, client *mcp.Client, url, session string) ([]json.RawMessage, error) {
	return client.ListAll(ctx, url, session, l.Method, l.ResultKey)
}
