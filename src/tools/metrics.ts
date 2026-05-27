import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { KibanaClient } from "../kibana-client.js";

export function registerMetricTools(server: McpServer, client: KibanaClient) {
  server.tool(
    "get_service_metrics",
    "Get runtime metric charts (CPU/memory, etc.) for a service. Returned metrics vary by agent (JVM: heap/non-heap/GC/thread; Node.js: RSS/heap/CPU, etc.)",
    {
      serviceName: z.string().describe("Service name"),
      agentName: z
        .string()
        .describe("APM agent type (e.g. java, nodejs, go, python, dotnet, ruby)"),
      start: z.string().describe("Query start time (ISO 8601)"),
      end: z.string().describe("Query end time (ISO 8601)"),
      environment: z.string().optional().describe("Environment filter"),
      serviceRuntimeName: z
        .string()
        .optional()
        .describe("Service runtime name (required on some Kibana versions, e.g. OpenJDK)"),
    },
    async ({ serviceName, agentName, start, end, environment, serviceRuntimeName }) => {
      const data = await client.getServiceMetrics(
        serviceName,
        agentName,
        start,
        end,
        environment,
        serviceRuntimeName
      );
      return {
        content: [{ type: "text" as const, text: JSON.stringify(data, null, 2) }],
      };
    }
  );
}
