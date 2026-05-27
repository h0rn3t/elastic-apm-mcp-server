#!/usr/bin/env node
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { KibanaClient } from "./kibana-client.js";
import { registerServiceTools } from "./tools/services.js";
import { registerErrorTools } from "./tools/errors.js";
import { registerTransactionTools } from "./tools/transactions.js";
import { registerMetricTools } from "./tools/metrics.js";
import { registerTraceTools } from "./tools/traces.js";

const kibanaUrl = process.env.KIBANA_URL;
const username = process.env.KIBANA_USERNAME;
const password = process.env.KIBANA_PASSWORD;

if (!kibanaUrl || !username || !password) {
  console.error("KIBANA_URL, KIBANA_USERNAME, KIBANA_PASSWORD environment variables are required");
  process.exit(1);
}

const client = new KibanaClient(kibanaUrl, username, password);

const server = new McpServer(
  {
    name: "elastic-apm-mcp-server",
    version: "1.0.0",
  },
  {
    instructions: `MCP server for Elastic APM via Kibana internal API.

Checking a service for errors (most common request):
- Use get_errors or check_service_errors with serviceName (APM service name, e.g. auth-service).
- Only serviceName is required; default time range is last 24h.
- For "last hour" / "past 6 hours" set period to 1h or 6h, or use hours: 3.
- If the service name is unknown, call list_services first, then get_errors.
- get_service_overview also returns an error summary plus transaction stats for one service.

Other tools:
- list_services — all APM services in a time range
- get_transactions — transaction latency stats (avg / p95 / p99)
- get_service_metrics — CPU/memory (depends on agent type)
- get_transaction_samples + get_trace — slow request / span bottleneck analysis (2-step)

Time ranges: ISO 8601 start/end, or period (15m, 1h, 6h, 24h, 7d), or hours (1–168).`,
  }
);

registerServiceTools(server, client);
registerErrorTools(server, client);
registerTransactionTools(server, client);
registerMetricTools(server, client);
registerTraceTools(server, client);

const transport = new StdioServerTransport();
await server.connect(transport);
