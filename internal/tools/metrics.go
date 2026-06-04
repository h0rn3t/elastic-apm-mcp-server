package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMetricTools(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_service_metrics",
		mcp.WithDescription("Get runtime metric charts (CPU/memory, etc.) for a service. Returned metrics vary by agent (JVM: heap/non-heap/GC/thread; Node.js: RSS/heap/CPU, etc.)"),
		mcp.WithString("serviceName",
			mcp.Description("Service name"),
			mcp.Required(),
		),
		mcp.WithString("agentName",
			mcp.Description("APM agent type (e.g. java, nodejs, go, python, dotnet, ruby)"),
			mcp.Required(),
		),
		mcp.WithString("start",
			mcp.Description("Query start time (ISO 8601)"),
			mcp.Required(),
		),
		mcp.WithString("end",
			mcp.Description("Query end time (ISO 8601)"),
			mcp.Required(),
		),
		mcp.WithString("environment",
			mcp.Description("Environment filter"),
		),
		mcp.WithString("serviceRuntimeName",
			mcp.Description("Service runtime name (required on some Kibana versions, e.g. OpenJDK)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serviceName := req.GetString("serviceName", "")
		agentName := req.GetString("agentName", "")
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		if serviceName == "" || agentName == "" || start == "" || end == "" {
			return mcp.NewToolResultError("serviceName, agentName, start and end are required"), nil
		}
		data, err := client.GetServiceMetrics(
			serviceName, agentName, start, end,
			req.GetString("environment", ""),
			req.GetString("serviceRuntimeName", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get metrics: %v", err)), nil
		}
		text, err := formatRawJSON(data)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}
