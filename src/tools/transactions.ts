import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { KibanaClient } from "../kibana-client.js";

export function registerTransactionTools(server: McpServer, client: KibanaClient) {
  server.tool(
    "get_transactions",
    "Get transaction performance statistics for a specific service",
    {
      serviceName: z.string().describe("Service name"),
      start: z.string().describe("Query start time (ISO 8601)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      transactionType: z
        .string()
        .default("request")
        .describe("Transaction type (e.g. request, messaging)"),
      environment: z.string().optional().describe("Environment filter"),
      latencyAggregationType: z
        .string()
        .default("avg")
        .describe(
          "Latency aggregation (avg, p95, p99). p95/p99 use sampled raw transactions, so transaction counts may appear lower"
        ),
    },
    async ({ serviceName, start, end, transactionType, environment, latencyAggregationType }) => {
      const data = await client.getTransactions(
        serviceName,
        start,
        end,
        transactionType,
        environment,
        latencyAggregationType
      );
      return {
        content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }],
      };
    }
  );
}
