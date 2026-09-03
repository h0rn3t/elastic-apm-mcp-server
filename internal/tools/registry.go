package tools

import (
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll реєструє всі MCP-інструменти на сервері.
func RegisterAll(s *server.MCPServer, client *kibana.Client) {
	registerServiceTools(s, client)
	registerErrorTools(s, client)
	registerTransactionTools(s, client)
	registerMetricTools(s, client)
	registerTraceTools(s, client)
	registerSearchTools(s, client)
}
