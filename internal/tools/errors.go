package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/timerange"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const errorToolDescription = "Check an Elastic APM service for errors and exceptions. Use when the user asks to verify, inspect, audit, or troubleshoot errors for a named service. Examples: 'check auth-service for errors', 'are there errors in payment-service in the last hour?'. Only serviceName is required; time range defaults to last 24h."

func errorToolParams() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("serviceName",
			mcp.Description("APM service name (exact name in Elastic APM), e.g. auth-service, payment-service"),
			mcp.Required(),
		),
		mcp.WithString("period",
			mcp.Description("Time window shorthand: 15m, 1h, 6h, 24h, 7d. Use when the user says 'last hour', 'past day'. Default: 24h if start/end/hours omitted"),
		),
		mcp.WithNumber("hours",
			mcp.Description("Custom lookback in hours (1–168). Alternative to period"),
		),
		mcp.WithString("start",
			mcp.Description("Range start (ISO 8601). Optional if period or hours is set"),
		),
		mcp.WithString("end",
			mcp.Description("Range end (ISO 8601). Defaults to now"),
		),
		mcp.WithString("environment",
			mcp.Description("Environment filter, e.g. production, staging"),
		),
		kueryParam(),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	}
}

func registerErrorTools(s *server.MCPServer, client *kibana.Client) {
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return fetchServiceErrorsMCP(client, req)
	}

	getErrors := mcp.NewTool("get_errors", append([]mcp.ToolOption{mcp.WithDescription(errorToolDescription)}, errorToolParams()...)...)
	s.AddTool(getErrors, handler)

	checkErrors := mcp.NewTool("check_service_errors",
		append([]mcp.ToolOption{mcp.WithDescription(`Alias for get_errors — same behavior. Prefer this when the user explicitly asks to "check" or "verify" a service for errors.`)}, errorToolParams()...)...,
	)
	s.AddTool(checkErrors, handler)

	registerErrorDetailTool(s, client)
}

func fetchServiceErrorsMCP(client *kibana.Client, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serviceName := req.GetString("serviceName", "")
	if serviceName == "" {
		return mcp.NewToolResultError("serviceName is required"), nil
	}

	args := readTimeArgs(req)
	if args.PeriodSet {
		if _, err := timerange.ParsePeriod(string(args.Period)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	rangeResolved, err := args.resolve(timerange.Period24h)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	raw, err := client.GetErrors(serviceName, rangeResolved.Start, rangeResolved.End, args.Environment, args.Kuery)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get errors: %v", err)), nil
	}

	summary := SummarizeServiceErrors(serviceName, rangeResolved, raw)
	text, err := formatJSON(summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

// FetchServiceErrorsCLI — для CLI-режиму.
func FetchServiceErrorsCLI(client *kibana.Client, serviceName string, args TimeArgs) (string, error) {
	rangeResolved, err := args.resolve(timerange.Period24h)
	if err != nil {
		return "", err
	}
	raw, err := client.GetErrors(serviceName, rangeResolved.Start, rangeResolved.End, args.Environment, args.Kuery)
	if err != nil {
		return "", err
	}
	summary := SummarizeServiceErrors(serviceName, rangeResolved, raw)
	return formatJSON(summary)
}
