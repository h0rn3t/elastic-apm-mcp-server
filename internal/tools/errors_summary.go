package tools

import (
	"encoding/json"
	"sort"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/timerange"
)

type errorGroupRow struct {
	Name          string `json:"name"`
	GroupID       string `json:"groupId,omitempty"`
	Message       string `json:"message,omitempty"`
	Type          string `json:"type,omitempty"`
	Occurrences   *int   `json:"occurrences,omitempty"`
	UsersAffected *int   `json:"usersAffected,omitempty"`
	LastSeen      string `json:"lastSeen,omitempty"`
}

type serviceErrorsSummary struct {
	Service   string `json:"service"`
	TimeRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Label string `json:"label"`
	} `json:"timeRange"`
	Summary struct {
		ErrorGroupCount  int  `json:"errorGroupCount"`
		TotalOccurrences *int `json:"totalOccurrences,omitempty"`
		HasErrors        bool `json:"hasErrors"`
	} `json:"summary"`
	ErrorGroups []errorGroupRow `json:"errorGroups"`
	Hint        string          `json:"hint"`
}

func pickNumber(values ...any) *int {
	for _, v := range values {
		switch n := v.(type) {
		case float64:
			i := int(n)
			return &i
		case int:
			return &n
		case json.Number:
			if f, err := n.Int64(); err == nil {
				i := int(f)
				return &i
			}
		}
	}
	return nil
}

func pickString(values ...any) string {
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func extractGroups(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var r map[string]any
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	if g, ok := r["errorGroups"].([]any); ok {
		return toMapSlice(g)
	}
	if g, ok := r["error_groups"].([]any); ok {
		return toMapSlice(g)
	}
	if cp, ok := r["currentPeriod"].(map[string]any); ok {
		if g, ok := cp["errorGroups"].([]any); ok {
			return toMapSlice(g)
		}
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return toMapSlice(arr)
	}
	return nil
}

func anySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}

// obj повертає вкладений об'єкт або nil-мапу, індексування якої безпечне —
// це дозволяє читати глибокі шляхи як obj(obj(src["http"])["response"]).
func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func toMapSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func mapGroup(g map[string]any) *errorGroupRow {
	if g == nil {
		return nil
	}
	var err map[string]any
	if e, ok := g["error"].(map[string]any); ok {
		err = e
	}
	var exc map[string]any
	if arr, ok := err["exception"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			exc = m
		}
	}

	name := pickString(g["name"], g["groupingName"], g["grouping_name"], err["grouping_name"])
	if name == "" {
		return nil
	}

	row := &errorGroupRow{
		Name:     name,
		GroupID:  pickString(g["groupId"], g["group_id"], g["id"], err["grouping_key"]),
		Message:  pickString(g["message"], exc["message"], err["message"]),
		Type:     pickString(g["type"], exc["type"], err["type"]),
		LastSeen: pickString(g["lastSeen"], g["latestOccurrenceAt"], g["@timestamp"]),
	}
	row.Occurrences = pickNumber(g["occurrences"], g["occurrenceCount"], g["error_count"], g["errorCount"], g["count"])
	row.UsersAffected = pickNumber(g["users"], g["affectedUserCount"], g["userCount"])
	return row
}

// SummarizeServiceErrors будує зручний для LLM підсумок помилок сервісу.
func SummarizeServiceErrors(serviceName string, tr timerange.Resolved, raw json.RawMessage) serviceErrorsSummary {
	groups := extractGroups(raw)
	rows := make([]errorGroupRow, 0, len(groups))
	for _, g := range groups {
		if row := mapGroup(g); row != nil {
			rows = append(rows, *row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		oi, oj := 0, 0
		if rows[i].Occurrences != nil {
			oi = *rows[i].Occurrences
		}
		if rows[j].Occurrences != nil {
			oj = *rows[j].Occurrences
		}
		return oi > oj
	})

	total := 0
	for _, r := range rows {
		if r.Occurrences != nil {
			total += *r.Occurrences
		}
	}

	var summary serviceErrorsSummary
	summary.Service = serviceName
	summary.TimeRange.Start = tr.Start
	summary.TimeRange.End = tr.End
	summary.TimeRange.Label = tr.Label
	summary.Summary.ErrorGroupCount = len(rows)
	summary.Summary.HasErrors = len(rows) > 0
	if total > 0 {
		summary.Summary.TotalOccurrences = &total
	}
	summary.ErrorGroups = rows
	if len(rows) == 0 {
		summary.Hint = "No error groups in this time range. Try a wider period (e.g. 7d) or list_services to confirm the exact service name."
	} else {
		summary.Hint = "For the culprit, exception chain and stacktrace of one group, call get_error_detail with its groupId."
	}
	return summary
}
