package tools

import (
	"cmp"
	"encoding/json"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
)

// transactionIndex — транзакції в data streams 8.x і в legacy-індексах 7.x.
const transactionIndex = "traces-apm*,apm-*-transaction*"

// APM API віддає зразки сотнями, і кожен потрапляє в контекст LLM, тому дефолт
// малий, а максимум обмежений.
const (
	DefaultSampleLimit = 50
	MaxSampleLimit     = 500
)

// sampleFields — те, чого не віддає /transactions/traces/samples, але без чого
// доводиться викликати get_trace лише щоб дізнатися «200 чи 403».
var sampleFields = []string{
	"@timestamp",
	"trace.id",
	"transaction.id",
	"transaction.duration.us",
	"transaction.outcome",
	"transaction.result",
	"http.response.status_code",
	"url.path",
	"url.full",
}

// SampleParams — параметри get_transaction_samples, спільні для MCP і CLI.
type SampleParams struct {
	Service         string
	Transaction     string
	Start           string
	End             string
	TransactionType string
	Environment     string
	Kuery           string
	SampleRangeFrom string
	SampleRangeTo   string
	Limit           int
}

type sampleRow struct {
	Timestamp     string `json:"timestamp,omitempty"`
	TraceID       string `json:"traceId"`
	TransactionID string `json:"transactionId,omitempty"`
	StatusCode    *int   `json:"statusCode,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
	Result        string `json:"result,omitempty"`
	URLPath       string `json:"urlPath,omitempty"`
	DurationMs    *int64 `json:"duration_ms,omitempty"`
}

type samplesResult struct {
	Service     string      `json:"service"`
	Transaction string      `json:"transaction"`
	Kuery       string      `json:"kuery,omitempty"`
	Count       int         `json:"count"`
	Available   int         `json:"availableCount,omitempty"`
	Samples     []sampleRow `json:"samples"`
	Note        string      `json:"note,omitempty"`
	Hint        string      `json:"hint,omitempty"`
}

// FetchTransactionSamples повертає зразки трейсів транзакції, доповнені
// статус-кодом, outcome, url.path і тривалістю — щоб вибрати потрібний сампл
// без окремого get_trace на кожен. Якщо доповнення недоступне (console proxy
// вимкнений або немає прав на індекси), зразки повертаються без цих полів, а
// причина — у note.
func FetchTransactionSamples(client *kibana.Client, p SampleParams) (string, error) {
	raw, err := client.GetTransactionSamples(
		p.Service, p.Transaction, p.Start, p.End,
		p.TransactionType, p.Environment,
		p.SampleRangeFrom, p.SampleRangeTo, p.Kuery,
	)
	if err != nil {
		return "", err
	}

	rows := parseTraceSamples(raw)
	// Обрізаємо до доповнення: доповнюємо лише те, що справді повернемо.
	available := len(rows)
	if limit := clampSampleLimit(p.Limit); available > limit {
		rows = rows[:limit]
	}

	result := samplesResult{
		Service:     p.Service,
		Transaction: p.Transaction,
		Kuery:       p.Kuery,
		Count:       len(rows),
		Samples:     rows,
	}

	if err := enrichSamples(client, result.Samples); err != nil {
		result.Note = "statusCode/outcome/urlPath unavailable (Elasticsearch enrichment failed): " + err.Error()
	}
	switch {
	case result.Count == 0:
		result.Hint = "No samples. Widen start/end, drop kuery, or check the transaction name against get_transactions."
	case available > result.Count:
		result.Available = available
		result.Hint = fmt.Sprintf(
			"Showing the first %d of %d samples. Narrow with kuery or sampleRangeFrom/To rather than raising limit (max %d) — every sample lands in the model context. Pass a traceId (plus transactionId as entryTransactionId) to get_trace for the span waterfall.",
			result.Count, available, MaxSampleLimit,
		)
	default:
		result.Hint = "Pass a traceId (plus transactionId as entryTransactionId) to get_trace for the span waterfall."
	}
	return formatJSON(result)
}

// clampSampleLimit тримає limit у межах [1, MaxSampleLimit]; 0 — дефолт.
func clampSampleLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultSampleLimit
	case limit > MaxSampleLimit:
		return MaxSampleLimit
	default:
		return limit
	}
}

// parseTraceSamples читає traceSamples з відповіді внутрішнього APM API.
func parseTraceSamples(raw json.RawMessage) []sampleRow {
	if len(raw) == 0 {
		return nil
	}
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	samples := toMapSlice(anySlice(resp["traceSamples"]))
	if len(samples) == 0 {
		samples = toMapSlice(anySlice(resp["samples"]))
	}
	rows := make([]sampleRow, 0, len(samples))
	for _, s := range samples {
		row := sampleRow{
			Timestamp:     pickString(s["@timestamp"], s["timestamp"]),
			TraceID:       pickString(s["traceId"], s["trace.id"], obj(s["trace"])["id"]),
			TransactionID: pickString(s["transactionId"], s["transaction.id"], obj(s["transaction"])["id"]),
		}
		if row.TraceID == "" && row.TransactionID == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

// enrichSamples доповнює rows на місці одним _search за transaction.id.
func enrichSamples(client *kibana.Client, rows []sampleRow) error {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.TransactionID != "" {
			ids = append(ids, r.TransactionID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	raw, err := client.Search(kibana.SearchRequest{
		Index:  transactionIndex,
		Size:   min(len(ids), kibana.MaxSearchSize),
		Query:  map[string]any{"terms": map[string]any{"transaction.id": ids}},
		Source: sampleFields,
	})
	if err != nil {
		return err
	}

	byID := make(map[string]sampleRow, len(ids))
	for _, src := range searchHits(raw) {
		if row := sampleFromSource(src); row.TransactionID != "" {
			byID[row.TransactionID] = row
		}
	}
	for i := range rows {
		if found, ok := byID[rows[i].TransactionID]; ok {
			mergeSample(&rows[i], found)
		}
	}
	return nil
}

func sampleFromSource(src map[string]any) sampleRow {
	tx := obj(src["transaction"])
	urlDoc := obj(src["url"])
	row := sampleRow{
		Timestamp:     str(src["@timestamp"]),
		TraceID:       str(obj(src["trace"])["id"]),
		TransactionID: str(tx["id"]),
		Outcome:       str(tx["outcome"]),
		Result:        str(tx["result"]),
		URLPath:       pickString(urlDoc["path"], urlDoc["full"]),
		StatusCode:    pickNumber(obj(obj(src["http"])["response"])["status_code"]),
	}
	if us := durationUs(tx["duration"]); us > 0 {
		ms := toMs(us)
		row.DurationMs = &ms
	}
	return row
}

// mergeSample доповнює порожні поля dst значеннями з ES, не перезаписуючи те,
// що вже повернув APM API.
func mergeSample(dst *sampleRow, src sampleRow) {
	dst.Timestamp = cmp.Or(dst.Timestamp, src.Timestamp)
	dst.TraceID = cmp.Or(dst.TraceID, src.TraceID)
	dst.Outcome = cmp.Or(dst.Outcome, src.Outcome)
	dst.Result = cmp.Or(dst.Result, src.Result)
	dst.URLPath = cmp.Or(dst.URLPath, src.URLPath)
	dst.StatusCode = cmp.Or(dst.StatusCode, src.StatusCode)
	dst.DurationMs = cmp.Or(dst.DurationMs, src.DurationMs)
}
