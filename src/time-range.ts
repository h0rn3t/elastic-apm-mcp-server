export const PERIODS = ["15m", "1h", "6h", "24h", "7d"] as const;
export type Period = (typeof PERIODS)[number];

const PERIOD_MS: Record<Period, number> = {
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
};

export type ResolvedTimeRange = {
  start: string;
  end: string;
  label: string;
};

export function resolveTimeRange(opts: {
  start?: string;
  end?: string;
  period?: Period;
  hours?: number;
  defaultPeriod?: Period;
}): ResolvedTimeRange {
  const endDate = opts.end ? new Date(opts.end) : new Date();
  if (Number.isNaN(endDate.getTime())) {
    throw new Error(`Invalid end time: ${opts.end}`);
  }

  if (opts.start && opts.end) {
    const startDate = new Date(opts.start);
    if (Number.isNaN(startDate.getTime())) {
      throw new Error(`Invalid start time: ${opts.start}`);
    }
    return {
      start: startDate.toISOString(),
      end: endDate.toISOString(),
      label: `${opts.start} → ${opts.end}`,
    };
  }

  if (opts.start && !opts.end) {
    const startDate = new Date(opts.start);
    if (Number.isNaN(startDate.getTime())) {
      throw new Error(`Invalid start time: ${opts.start}`);
    }
    return {
      start: startDate.toISOString(),
      end: endDate.toISOString(),
      label: `from ${opts.start} to now`,
    };
  }

  let windowMs: number;
  if (opts.period) {
    windowMs = PERIOD_MS[opts.period];
  } else if (opts.hours != null) {
    if (opts.hours <= 0 || opts.hours > 24 * 7) {
      throw new Error("hours must be between 0 and 168 (7 days)");
    }
    windowMs = opts.hours * 60 * 60 * 1000;
  } else {
    const fallback = opts.defaultPeriod ?? "24h";
    windowMs = PERIOD_MS[fallback];
  }

  const startDate = new Date(endDate.getTime() - windowMs);
  const label = opts.period
    ? `last ${opts.period}`
    : opts.hours != null
      ? `last ${opts.hours}h`
      : `last ${opts.defaultPeriod ?? "24h"}`;

  return {
    start: startDate.toISOString(),
    end: endDate.toISOString(),
    label,
  };
}
