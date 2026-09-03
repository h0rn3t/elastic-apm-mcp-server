package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTraceTools(s *server.MCPServer, client *kibana.Client) {
	registerGetTransactionSamples(s, client)
	registerGetTrace(s, client)
}

func registerGetTransactionSamples(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_transaction_samples",
		mcp.WithDescription("Step 1 of span detail / slow-request analysis — find sample traces for a transaction. Each sample carries traceId, transactionId, http.response.status_code, transaction.outcome, url.path and duration, so \"which of these requests returned 403?\" is answered here without calling get_trace. Narrow with kuery (e.g. 'http.response.status_code : 403') or latency with sampleRangeFrom/To (microseconds); pass a traceId to get_trace only for span-level bottleneck analysis"),
		mcp.WithString("serviceName",
			mcp.Description("Service name"),
			mcp.Required(),
		),
		mcp.WithString("transactionName",
			mcp.Description("Transaction name (key from get_transactions/get_service_overview results)"),
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
		mcp.WithString("sampleRangeFrom",
			mcp.Description("Sample latency lower bound (microseconds). Set to filter slow transactions only"),
		),
		mcp.WithString("sampleRangeTo",
			mcp.Description("Sample latency upper bound (microseconds)"),
		),
		mcp.WithNumber("limit",
			mcp.Description(fmt.Sprintf("Max samples returned (1–%d, default %d). Every sample lands in the model context, so narrow with kuery or sampleRangeFrom/To instead of raising this", MaxSampleLimit, DefaultSampleLimit)),
		),
		kueryParam(),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serviceName := req.GetString("serviceName", "")
		transactionName := req.GetString("transactionName", "")
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		if serviceName == "" || transactionName == "" || start == "" || end == "" {
			return mcp.NewToolResultError("serviceName, transactionName, start and end are required"), nil
		}
		text, err := FetchTransactionSamples(client, SampleParams{
			Service:         serviceName,
			Transaction:     transactionName,
			Start:           start,
			End:             end,
			TransactionType: req.GetString("transactionType", "request"),
			Environment:     req.GetString("environment", ""),
			Kuery:           req.GetString("kuery", ""),
			SampleRangeFrom: req.GetString("sampleRangeFrom", ""),
			SampleRangeTo:   req.GetString("sampleRangeTo", ""),
			Limit:           req.GetInt("limit", 0),
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get transaction samples: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}

func registerGetTrace(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_trace",
		mcp.WithDescription(`Span detail / transaction bottleneck analysis — fetch the span waterfall for one trace. Use to answer "where is the bottleneck in a slow request?" or "which segment spent time (DB, external call, internal)?". Response is a parent-child tree (children sorted by duration desc) plus top 5 bottlenecks by self time (topBottlenecks). Obtain traceId from get_transaction_samples`),
		mcp.WithString("traceId",
			mcp.Description("Trace ID"),
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
		mcp.WithString("entryTransactionId",
			mcp.Description("Entry transaction ID (required on some Kibana versions; transactionId from get_transaction_samples)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		traceID := req.GetString("traceId", "")
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		if traceID == "" || start == "" || end == "" {
			return mcp.NewToolResultError("traceId, start and end are required"), nil
		}
		raw, err := client.GetTrace(traceID, start, end, req.GetString("entryTransactionId", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get trace: %v", err)), nil
		}
		summary, err := SummarizeTrace(raw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("summarize trace: %v", err)), nil
		}
		return mcp.NewToolResultText(string(summary)), nil
	})
}
