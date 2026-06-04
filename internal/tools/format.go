package tools

import "encoding/json"

func formatJSON(v any) (string, error) {
	return FormatJSON(v)
}

// FormatJSON форматує значення як відформатований JSON.
func FormatJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatRawJSON(raw json.RawMessage) (string, error) {
	return FormatRawJSON(raw)
}

// FormatRawJSON форматує сирий JSON.
func FormatRawJSON(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw), nil
	}
	return FormatJSON(v)
}

// SummarizeTraceText будує підсумок waterfall для CLI.
func SummarizeTraceText(raw json.RawMessage) (string, error) {
	b, err := SummarizeTrace(raw)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
