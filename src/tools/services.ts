import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { KibanaClient } from "../kibana-client.js";

export function registerServiceTools(server: McpServer, client: KibanaClient) {
  server.tool(
    "list_services",
    "List services registered in APM (latency is always avg; use get_service_overview/get_transactions for percentiles)",
    {
      start: z.string().describe("Query start time (ISO 8601, e.g. 2026-04-01T00:00:00Z)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      environment: z.string().optional().describe("Environment filter (e.g. production, staging)"),
    },
    async ({ start, end, environment }) => {
      const data = await client.getServices(start, end, environment);
      return {
        content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }],
      };
    }
  );

  server.tool(
    "get_service_overview",
    "Get transaction performance and error summary for a specific service",
    {
      serviceName: z.string().describe("Service name"),
      start: z.string().describe("Query start time (ISO 8601)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      environment: z.string().optional().describe("Environment filter"),
      latencyAggregationType: z
        .string()
        .default("avg")
        .describe(
          "Latency aggregation (avg, p95, p99). p95/p99 use sampled raw transactions, so transaction counts may appear lower"
        ),
    },
    async ({ serviceName, start, end, environment, latencyAggregationType }) => {
      const data = await client.getServiceOverview(
        serviceName,
        start,
        end,
        environment,
        latencyAggregationType
      );
      return {
        content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }],
      };
    }
  );
}
