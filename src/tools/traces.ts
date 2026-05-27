import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { KibanaClient } from "../kibana-client.js";
import { summarizeTrace } from "../trace-waterfall.js";

export function registerTraceTools(server: McpServer, client: KibanaClient) {
  server.tool(
    "get_transaction_samples",
    "Step 1 of span detail / slow-request analysis — find sample traces (traceId/transactionId) for a transaction. Narrow latency with sampleRangeFrom/To (microseconds) to pick slow requests only; pass the traceId to get_trace for span-level bottleneck analysis",
    {
      serviceName: z.string().describe("Service name"),
      transactionName: z
        .string()
        .describe("Transaction name (key from get_transactions/get_service_overview results)"),
      start: z.string().describe("Query start time (ISO 8601)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      transactionType: z
        .string()
        .default("request")
        .describe("Transaction type (e.g. request, messaging)"),
      environment: z.string().optional().describe("Environment filter"),
      sampleRangeFrom: z
        .string()
        .optional()
        .describe("Sample latency lower bound (microseconds). Set to filter slow transactions only"),
      sampleRangeTo: z
        .string()
        .optional()
        .describe("Sample latency upper bound (microseconds)"),
    },
    async ({
      serviceName,
      transactionName,
      start,
      end,
      transactionType,
      environment,
      sampleRangeFrom,
      sampleRangeTo,
    }) => {
      const data = await client.getTransactionSamples(
        serviceName,
        transactionName,
        start,
        end,
        transactionType,
        environment,
        sampleRangeFrom,
        sampleRangeTo
      );
      return {
        content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }],
      };
    }
  );

  server.tool(
    "get_trace",
    'Span detail / transaction bottleneck analysis — fetch the span waterfall for one trace. Use to answer "where is the bottleneck in a slow request?" or "which segment spent time (DB, external call, internal)?". Response is a parent-child tree (children sorted by duration desc) plus top 5 bottlenecks by self time (topBottlenecks). Obtain traceId from get_transaction_samples',
    {
      traceId: z.string().describe("Trace ID"),
      start: z.string().describe("Query start time (ISO 8601)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      entryTransactionId: z
        .string()
        .optional()
        .describe(
          "Entry transaction ID (required on some Kibana versions; transactionId from get_transaction_samples)"
        ),
    },
    async ({ traceId, start, end, entryTransactionId }) => {
      const data = await client.getTrace(traceId, start, end, entryTransactionId);
      const summary = summarizeTrace(data);
      return {
        content: [{ type: "text" as const, text: JSON.stringify(summary, null, 2) }],
      };
    }
  );
}
