package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTransactionTools(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_transactions",
		mcp.WithDescription("Get transaction performance statistics for a specific service"),
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
		mcp.WithString("transactionType",
			mcp.Description("Transaction type (e.g. request, messaging)"),
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
		txType := req.GetString("transactionType", "request")
		latency := req.GetString("latencyAggregationType", "avg")
		data, err := client.GetTransactions(serviceName, start, end, txType, req.GetString("environment", ""), latency)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get transactions: %v", err)), nil
		}
		text, err := formatRawJSON(data)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}
