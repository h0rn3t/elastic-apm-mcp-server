package tools

import (
	"context"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerVersionTool реєструє get_version: діагностика підключення.
func registerVersionTool(s *server.MCPServer, client *kibana.Client, binaryVersion string) {
	tool := mcp.NewTool("get_version",
		mcp.WithDescription("Server and Kibana versions plus connectivity check. Use first when APM tools fail — tells whether Kibana is reachable, which version it runs, and which binary serves the request"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, err := FetchVersion(client, binaryVersion)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}

// FetchVersion збирає версію бінарника та версію Kibana через /api/status.
// Помилка Kibana не є фатальною: повертається тим самим JSON з полем error.
func FetchVersion(client *kibana.Client, binaryVersion string) (string, error) {
	kibanaVersion, err := client.GetKibanaVersion()
	out := map[string]any{"binaryVersion": binaryVersion}
	if err != nil {
		out["kibana"] = "unreachable"
		out["error"] = err.Error()
		out["hint"] = "Check KIBANA_URL/KIBANA_USERNAME/KIBANA_PASSWORD and that Kibana answers GET /api/status"
		return formatJSON(out)
	}
	out["kibanaVersion"] = kibanaVersion
	return formatJSON(out)
}
