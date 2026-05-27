import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { KibanaClient } from "../kibana-client.js";
import { summarizeServiceErrors } from "../errors-summary.js";
import { PERIODS, resolveTimeRange, type Period } from "../time-range.js";

const errorToolSchema = {
  serviceName: z
    .string()
    .describe(
      "APM service name (exact name in Elastic APM), e.g. auth-service, payment-service"
    ),
  period: z
    .enum(PERIODS)
    .optional()
    .describe(
      "Time window shorthand: 15m, 1h, 6h, 24h, 7d. Use when the user says 'last hour', 'past day'. Default: 24h if start/end/hours omitted"
    ),
  hours: z
    .number()
    .positive()
    .max(168)
    .optional()
    .describe("Custom lookback in hours (1–168). Alternative to period"),
  start: z
    .string()
    .optional()
    .describe("Range start (ISO 8601). Optional if period or hours is set"),
  end: z
    .string()
    .optional()
    .describe("Range end (ISO 8601). Defaults to now"),
  environment: z
    .string()
    .optional()
    .describe("Environment filter, e.g. production, staging"),
};

const errorToolDescription =
  "Check an Elastic APM service for errors and exceptions. Use when the user asks to verify, inspect, audit, or troubleshoot errors for a named service. Examples: 'check auth-service for errors', 'are there errors in payment-service in the last hour?'. Only serviceName is required; time range defaults to last 24h.";

async function fetchServiceErrors(
  client: KibanaClient,
  args: {
    serviceName: string;
    period?: Period;
    hours?: number;
    start?: string;
    end?: string;
    environment?: string;
  }
) {
  const range = resolveTimeRange({
    start: args.start,
    end: args.end,
    period: args.period,
    hours: args.hours,
    defaultPeriod: "24h",
  });

  const raw = await client.getErrors(
    args.serviceName,
    range.start,
    range.end,
    args.environment
  );

  const summary = summarizeServiceErrors(args.serviceName, range, raw);

  return {
    content: [
      {
        type: "text" as const,
        text: JSON.stringify(summary, null, 2),
      },
    ],
  };
}

export function registerErrorTools(server: McpServer, client: KibanaClient) {
  const handler = async (args: {
    serviceName: string;
    period?: Period;
    hours?: number;
    start?: string;
    end?: string;
    environment?: string;
  }) => fetchServiceErrors(client, args);

  server.tool("get_errors", errorToolDescription, errorToolSchema, handler);

  server.tool(
    "check_service_errors",
    `Alias for get_errors — same behavior. Prefer this when the user explicitly asks to "check" or "verify" a service for errors.`,
    errorToolSchema,
    handler
  );
}
