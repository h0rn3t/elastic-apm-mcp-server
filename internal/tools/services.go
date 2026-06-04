package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerServiceTools(s *server.MCPServer, client *kibana.Client) {
	registerListServices(s, client)
	registerGetServiceOverview(s, client)
}

func registerListServices(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("list_services",
		mcp.WithDescription("List services registered in APM (latency is always avg; use get_service_overview/get_transactions for percentiles)"),
		mcp.WithString("start",
			mcp.Description("Query start time (ISO 8601, e.g. 2026-04-01T00:00:00Z)"),
			mcp.Required(),
		),
		mcp.WithString("end",
			mcp.Description("Query end time (ISO 8601)"),
			mcp.Required(),
		),
		mcp.WithString("environment",
			mcp.Description("Environment filter (e.g. production, staging)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		if start == "" || end == "" {
			return mcp.NewToolResultError("start and end are required"), nil
		}
		data, err := client.GetServices(start, end, req.GetString("environment", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list services: %v", err)), nil
		}
		text, err := formatRawJSON(data)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}

func registerGetServiceOverview(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_service_overview",
		mcp.WithDescription("Get transaction performance and error summary for a specific service"),
		mcp.WithString("serviceName",
			mcp.Description("Service name"),
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
		mcp.WithString("latencyAggregationType",
			mcp.Description("Latency aggregation (avg, p95, p99). p95/p99 use sampled raw transactions, so transaction counts may appear lower"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serviceName := req.GetString("serviceName", "")
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		if serviceName == "" || start == "" || end == "" {
			return mcp.NewToolResultError("serviceName, start and end are required"), nil
		}
		latency := req.GetString("latencyAggregationType", "avg")
		data, err := client.GetServiceOverview(serviceName, start, end, req.GetString("environment", ""), latency)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("service overview: %v", err)), nil
		}
		text, err := formatJSON(data)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}
