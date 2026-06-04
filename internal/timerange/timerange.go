package timerange

import (
	"fmt"
	"time"
)

// Period — скорочення для вікна запиту.
type Period string

const (
	Period15m Period = "15m"
	Period1h  Period = "1h"
	Period6h  Period = "6h"
	Period24h Period = "24h"
	Period7d  Period = "7d"
)

var periodMS = map[Period]time.Duration{
	Period15m: 15 * time.Minute,
	Period1h:  time.Hour,
	Period6h:  6 * time.Hour,
	Period24h: 24 * time.Hour,
	Period7d:  7 * 24 * time.Hour,
}

// ValidPeriods — дозволені значення period.
var ValidPeriods = []Period{Period15m, Period1h, Period6h, Period24h, Period7d}

// Resolved — розв'язаний часовий діапазон.
type Resolved struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Label string `json:"label"`
}

// Options — параметри для Resolve.
type Options struct {
	Start         string
	End           string
	Period        Period
	Hours         float64
	HoursSet      bool
	DefaultPeriod Period
}

// Resolve обчислює ISO 8601 start/end з явних дат, period або hours.
func Resolve(opts Options) (Resolved, error) {
	endDate := time.Now().UTC()
	if opts.End != "" {
		var err error
		endDate, err = parseTime(opts.End)
		if err != nil {
			return Resolved{}, fmt.Errorf("invalid end time: %s", opts.End)
		}
	}

	if opts.Start != "" && opts.End != "" {
		startDate, err := parseTime(opts.Start)
		if err != nil {
			return Resolved{}, fmt.Errorf("invalid start time: %s", opts.Start)
		}
		return Resolved{
			Start: startDate.UTC().Format(time.RFC3339Nano),
			End:   endDate.UTC().Format(time.RFC3339Nano),
			Label: fmt.Sprintf("%s → %s", opts.Start, opts.End),
		}, nil
	}

	if opts.Start != "" && opts.End == "" {
		startDate, err := parseTime(opts.Start)
		if err != nil {
			return Resolved{}, fmt.Errorf("invalid start time: %s", opts.Start)
		}
		return Resolved{
			Start: startDate.UTC().Format(time.RFC3339Nano),
			End:   endDate.UTC().Format(time.RFC3339Nano),
			Label: fmt.Sprintf("from %s to now", opts.Start),
		}, nil
	}

	var window time.Duration
	switch {
	case opts.Period != "":
		d, ok := periodMS[opts.Period]
		if !ok {
			return Resolved{}, fmt.Errorf("invalid period: %s", opts.Period)
		}
		window = d
	case opts.HoursSet:
		if opts.Hours <= 0 || opts.Hours > 168 {
			return Resolved{}, fmt.Errorf("hours must be between 0 and 168 (7 days)")
		}
		window = time.Duration(opts.Hours * float64(time.Hour))
	default:
		fallback := opts.DefaultPeriod
		if fallback == "" {
			fallback = Period24h
		}
		window = periodMS[fallback]
	}

	startDate := endDate.Add(-window)
	label := fmt.Sprintf("last %s", fallbackLabel(opts))
	return Resolved{
		Start: startDate.UTC().Format(time.RFC3339Nano),
		End:   endDate.UTC().Format(time.RFC3339Nano),
		Label: label,
	}, nil
}

func fallbackLabel(opts Options) string {
	if opts.Period != "" {
		return string(opts.Period)
	}
	if opts.HoursSet {
		return fmt.Sprintf("%.0fh", opts.Hours)
	}
	p := opts.DefaultPeriod
	if p == "" {
		p = Period24h
	}
	return string(p)
}

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// ParsePeriod перевіряє рядок period.
func ParsePeriod(s string) (Period, error) {
	p := Period(s)
	if _, ok := periodMS[p]; !ok {
		return "", fmt.Errorf("period must be one of: 15m, 1h, 6h, 24h, 7d")
	}
	return p, nil
}
