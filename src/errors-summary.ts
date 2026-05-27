import type { ResolvedTimeRange } from "./time-range.js";

type ErrorGroupRow = {
  name: string;
  message?: string;
  type?: string;
  occurrences?: number;
  usersAffected?: number;
  lastSeen?: string;
};

function pickNumber(...values: unknown[]): number | undefined {
  for (const v of values) {
    if (typeof v === "number" && !Number.isNaN(v)) return v;
  }
  return undefined;
}

function pickString(...values: unknown[]): string | undefined {
  for (const v of values) {
    if (typeof v === "string" && v.length > 0) return v;
  }
  return undefined;
}

function extractGroups(raw: unknown): unknown[] {
  if (!raw || typeof raw !== "object") return [];
  const r = raw as Record<string, unknown>;
  if (Array.isArray(r.errorGroups)) return r.errorGroups;
  if (Array.isArray(r.error_groups)) return r.error_groups;
  if (r.currentPeriod && typeof r.currentPeriod === "object") {
    const cp = r.currentPeriod as Record<string, unknown>;
    if (Array.isArray(cp.errorGroups)) return cp.errorGroups;
  }
  if (Array.isArray(raw)) return raw;
  return [];
}

function mapGroup(g: unknown): ErrorGroupRow | null {
  if (!g || typeof g !== "object") return null;
  const row = g as Record<string, unknown>;
  const err =
    row.error && typeof row.error === "object" ? (row.error as Record<string, unknown>) : undefined;
  const exc = Array.isArray(err?.exception)
    ? (err!.exception as Record<string, unknown>[])[0]
    : undefined;

  const name = pickString(row.name, row.groupingName, row.grouping_name, err?.grouping_name);
  if (!name) return null;

  return {
    name,
    message: pickString(row.message, exc?.message, err?.message),
    type: pickString(row.type, exc?.type, err?.type),
    occurrences: pickNumber(
      row.occurrences,
      row.occurrenceCount,
      row.error_count,
      row.errorCount,
      row.count
    ),
    usersAffected: pickNumber(row.users, row.affectedUserCount, row.userCount),
    lastSeen: pickString(row.lastSeen, row.latestOccurrenceAt, row["@timestamp"]),
  };
}

export function summarizeServiceErrors(
  serviceName: string,
  range: ResolvedTimeRange,
  raw: unknown
) {
  const groups = extractGroups(raw)
    .map(mapGroup)
    .filter((g): g is ErrorGroupRow => g != null)
    .sort((a, b) => (b.occurrences ?? 0) - (a.occurrences ?? 0));

  const totalOccurrences = groups.reduce((sum, g) => sum + (g.occurrences ?? 0), 0);

  return {
    service: serviceName,
    timeRange: {
      start: range.start,
      end: range.end,
      label: range.label,
    },
    summary: {
      errorGroupCount: groups.length,
      totalOccurrences: totalOccurrences || undefined,
      hasErrors: groups.length > 0,
    },
    errorGroups: groups,
    hint:
      groups.length === 0
        ? "No error groups in this time range. Try a wider period (e.g. 7d) or list_services to confirm the exact service name."
        : "For details on a specific error, open APM → Errors in Kibana or use get_service_overview.",
  };
}
