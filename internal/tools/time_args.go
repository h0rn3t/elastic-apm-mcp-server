package tools

import (
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/timerange"
	"github.com/mark3labs/mcp-go/mcp"
)

// TimeArgs — часові параметри запиту.
type TimeArgs struct {
	Start       string
	End         string
	Period      timerange.Period
	PeriodSet   bool
	Hours       float64
	HoursSet    bool
	Environment string
}

func readTimeArgs(req mcp.CallToolRequest) TimeArgs {
	period := req.GetString("period", "")
	args := TimeArgs{
		Start:       req.GetString("start", ""),
		End:         req.GetString("end", ""),
		Environment: req.GetString("environment", ""),
	}
	if period != "" {
		args.Period = timerange.Period(period)
		args.PeriodSet = true
	}
	h := req.GetFloat("hours", 0)
	if h > 0 {
		args.Hours = h
		args.HoursSet = true
	}
	return args
}

// ReadTimeArgsCLI зчитує часові параметри з CLI-прапорців.
func ReadTimeArgsCLI(start, end, period string, hours float64, hoursSet bool) (TimeArgs, error) {
	args := TimeArgs{Start: start, End: end, Hours: hours, HoursSet: hoursSet}
	if period != "" {
		p, err := timerange.ParsePeriod(period)
		if err != nil {
			return TimeArgs{}, err
		}
		args.Period = p
		args.PeriodSet = true
	}
	return args, nil
}

func (a TimeArgs) resolve(defaultPeriod timerange.Period) (timerange.Resolved, error) {
	opts := timerange.Options{
		Start:         a.Start,
		End:           a.End,
		Hours:         a.Hours,
		HoursSet:      a.HoursSet,
		DefaultPeriod: defaultPeriod,
	}
	if a.PeriodSet {
		opts.Period = a.Period
	}
	return timerange.Resolve(opts)
}

// resolveRequiredTimeRange — для інструментів, де start/end обов'язкові в схемі MCP.
func resolveRequiredTimeRange(start, end string) (timerange.Resolved, error) {
	return timerange.Resolve(timerange.Options{Start: start, End: end})
}
