package main

import (
	"fmt"
	"os"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/cli"
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

var version = "0.2.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run перемикає між MCP stdio (за замовчуванням) і CLI-режимом.
func run(args []string) int {
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version" || args[0] == "-V") {
		fmt.Println(version)
		return 0
	}

	client, err := kibana.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kibana client: %v\n", err)
		return 1
	}

	if len(args) == 0 || args[0] == "mcp" || args[0] == "serve" {
		return runMCP(client)
	}
	return cli.Run(args, client)
}

func runMCP(client *kibana.Client) int {
	s := server.NewMCPServer(
		"elastic-apm-mcp-server",
		version,
		server.WithInstructions(`MCP server for Elastic APM via Kibana internal API.

Checking a service for errors (most common request):
- Use get_errors or check_service_errors with serviceName (APM service name, e.g. auth-service).
- Only serviceName is required; default time range is last 24h.
- For "last hour" / "past 6 hours" set period to 1h or 6h, or use hours: 3.
- If the service name is unknown, call list_services first, then get_errors.
- get_service_overview also returns an error summary plus transaction stats for one service.
- For the culprit, exception chain and stacktrace of one group, pass its errorGroups[].groupId to get_error_detail.

Other tools:
- list_services — all APM services in a time range
- get_transactions — transaction latency stats (avg / p95 / p99)
- get_service_metrics — CPU/memory (depends on agent type)
- get_transaction_samples — samples with status code, outcome, url.path and duration
- get_trace — span waterfall for one traceId; only needed for bottleneck analysis
- get_error_detail — culprit, exception chain, stacktrace of one error group
- search_raw — raw Elasticsearch _search with optional aggs for group-by analytics
- get_version — binary and Kibana versions; run first when APM tools fail

Narrowing results — every tool except get_trace and search_raw takes kuery, a KQL filter
Elasticsearch applies server-side. Prefer it over fetching everything and filtering afterwards:
- kuery: 'http.response.status_code : 403' — which requests failed
- kuery: 'http.request.headers.True-Client-Ip : "95.67.96.24"' — one client's traffic
- kuery: 'transaction.outcome : failure and transaction.duration.us > 5000000'

Time ranges: ISO 8601 start/end, or period (15m, 1h, 6h, 24h, 7d), or hours (1–168).`),
	)

	tools.RegisterAll(s, client, version)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
