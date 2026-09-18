package tools

import (
	"encoding/json"
	"testing"
)

func TestSearchRawToolArraySchemasHaveItems(t *testing.T) {
	tool := searchRawTool()
	for _, name := range []string{"_source", "sort"} {
		property, ok := tool.InputSchema.Properties[name]
		if !ok {
			t.Fatalf("%s property is missing", name)
		}
		schema, ok := property.(map[string]any)
		if !ok {
			t.Fatalf("%s schema has unexpected type %T", name, property)
		}
		// Клієнти (напр. VSCode) відхиляють масив без типізованого items,
		// тому порожнього map[string]any{} тут недостатньо.
		items, ok := schema["items"].(map[string]any)
		if !ok {
			t.Errorf("%s schema has no items object: %#v", name, property)
			continue
		}
		if items["type"] == nil {
			t.Errorf("%s items schema has no type: %#v", name, items)
		}
	}
}

func TestParseTraceSamples(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []sampleRow
	}{
		{
			name: "kibana shape",
			raw:  `{"traceSamples":[{"traceId":"t1","transactionId":"x1"},{"traceId":"t2","transactionId":"x2"}]}`,
			want: []sampleRow{{TraceID: "t1", TransactionID: "x1"}, {TraceID: "t2", TransactionID: "x2"}},
		},
		{
			name: "with timestamp",
			raw:  `{"traceSamples":[{"@timestamp":"2026-09-03T10:00:00Z","traceId":"t1","transactionId":"x1"}]}`,
			want: []sampleRow{{Timestamp: "2026-09-03T10:00:00Z", TraceID: "t1", TransactionID: "x1"}},
		},
		{
			name: "nested field names",
			raw:  `{"samples":[{"trace":{"id":"t1"},"transaction":{"id":"x1"}}]}`,
			want: []sampleRow{{TraceID: "t1", TransactionID: "x1"}},
		},
		{
			name: "entries without ids are dropped",
			raw:  `{"traceSamples":[{"score":1},{"traceId":"t1"}]}`,
			want: []sampleRow{{TraceID: "t1"}},
		},
		{name: "empty response", raw: `{}`, want: []sampleRow{}},
		{name: "not json", raw: `<html>`, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTraceSamples(json.RawMessage(tt.raw))
			if len(got) != len(tt.want) {
				t.Fatalf("got %d samples, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("sample %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSampleFromSource(t *testing.T) {
	const doc = `{
		"@timestamp": "2026-09-03T10:00:00Z",
		"trace": {"id": "t1"},
		"transaction": {"id": "x1", "outcome": "failure", "result": "HTTP 4xx", "duration": {"us": 1500000}},
		"http": {"response": {"status_code": 403}},
		"url": {"path": "/ms-auth/api/v1/jwt/otp", "full": "https://host/ms-auth/api/v1/jwt/otp"}
	}`
	var src map[string]any
	if err := json.Unmarshal([]byte(doc), &src); err != nil {
		t.Fatal(err)
	}

	got := sampleFromSource(src)
	if got.StatusCode == nil || *got.StatusCode != 403 {
		t.Errorf("statusCode = %v, want 403", got.StatusCode)
	}
	if got.Outcome != "failure" {
		t.Errorf("outcome = %q, want failure", got.Outcome)
	}
	if got.URLPath != "/ms-auth/api/v1/jwt/otp" {
		t.Errorf("urlPath = %q, want the path, not the full URL", got.URLPath)
	}
	if got.DurationMs == nil || *got.DurationMs != 1500 {
		t.Errorf("duration_ms = %v, want 1500", got.DurationMs)
	}
	if got.TraceID != "t1" || got.TransactionID != "x1" {
		t.Errorf("ids = %q/%q, want t1/x1", got.TraceID, got.TransactionID)
	}
}

func TestSampleFromSourceMissingFields(t *testing.T) {
	var src map[string]any
	if err := json.Unmarshal([]byte(`{"transaction":{"id":"x1"}}`), &src); err != nil {
		t.Fatal(err)
	}
	got := sampleFromSource(src)
	if got.TransactionID != "x1" {
		t.Fatalf("transactionId = %q, want x1", got.TransactionID)
	}
	if got.StatusCode != nil || got.DurationMs != nil || got.URLPath != "" {
		t.Errorf("absent fields must stay zero, got %+v", got)
	}
}

func TestMergeSampleKeepsApmValues(t *testing.T) {
	status := 403
	dst := sampleRow{Timestamp: "from-apm", TraceID: "t1", TransactionID: "x1"}
	mergeSample(&dst, sampleRow{
		Timestamp:  "from-es",
		TraceID:    "other",
		Outcome:    "failure",
		URLPath:    "/otp",
		StatusCode: &status,
	})

	if dst.Timestamp != "from-apm" {
		t.Errorf("timestamp = %q, want the APM value preserved", dst.Timestamp)
	}
	if dst.TraceID != "t1" {
		t.Errorf("traceId = %q, want t1 preserved", dst.TraceID)
	}
	if dst.Outcome != "failure" || dst.URLPath != "/otp" {
		t.Errorf("empty fields were not filled from ES: %+v", dst)
	}
	if dst.StatusCode == nil || *dst.StatusCode != 403 {
		t.Errorf("statusCode = %v, want 403", dst.StatusCode)
	}
}

func TestSearchHits(t *testing.T) {
	raw := json.RawMessage(`{"took":3,"hits":{"total":{"value":2},"hits":[
		{"_id":"a","_source":{"transaction":{"id":"x1"}}},
		{"_id":"b","_source":{"transaction":{"id":"x2"}}},
		{"_id":"c"}
	]}}`)
	hits := searchHits(raw)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2 (the hit without _source is skipped)", len(hits))
	}
	if id := str(obj(hits[0]["transaction"])["id"]); id != "x1" {
		t.Errorf("first hit transaction.id = %q, want x1", id)
	}
	if hits := searchHits(json.RawMessage(`not json`)); hits != nil {
		t.Errorf("searchHits on invalid JSON = %v, want nil", hits)
	}
}
