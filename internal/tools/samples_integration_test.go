package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
)

// fakeKibana піднімає сервер, що відповідає і на внутрішній APM API, і на
// console proxy, та повертає клієнт, налаштований на нього.
func fakeKibana(t *testing.T, h http.HandlerFunc) *kibana.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	t.Setenv("KIBANA_URL", srv.URL)
	t.Setenv("KIBANA_USERNAME", "u")
	t.Setenv("KIBANA_PASSWORD", "p")
	client, err := kibana.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestFetchTransactionSamplesEnriches(t *testing.T) {
	var gotKuery string
	var proxyCalls int

	client := fakeKibana(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transactions/traces/samples"):
			gotKuery = r.URL.Query().Get("kuery")
			_, _ = w.Write([]byte(`{"traceSamples":[
				{"traceId":"t1","transactionId":"x1"},
				{"traceId":"t2","transactionId":"x2"}
			]}`))
		case r.URL.Path == "/api/console/proxy":
			proxyCalls++
			_, _ = w.Write([]byte(`{"hits":{"hits":[
				{"_source":{"trace":{"id":"t1"},"transaction":{"id":"x1","outcome":"failure","duration":{"us":900000}},
				 "http":{"response":{"status_code":403}},"url":{"path":"/otp"}}},
				{"_source":{"trace":{"id":"t2"},"transaction":{"id":"x2","outcome":"success","duration":{"us":120000}},
				 "http":{"response":{"status_code":200}},"url":{"path":"/otp"}}}
			]}}`))
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
		}
	})

	const kuery = `http.response.status_code : 403`
	text, err := FetchTransactionSamples(client, SampleParams{
		Service:     "ms-auth",
		Transaction: "POST /ms-auth/api/v1/jwt/otp",
		Start:       "2026-09-03T00:00:00Z",
		End:         "2026-09-03T01:00:00Z",
		Kuery:       kuery,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotKuery != kuery {
		t.Errorf("kuery reaching Kibana = %q, want %q", gotKuery, kuery)
	}
	if proxyCalls != 1 {
		t.Errorf("console proxy called %d times, want 1 for all samples", proxyCalls)
	}

	var got samplesResult
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if got.Note != "" {
		t.Errorf("note = %q, want empty on success", got.Note)
	}
	if got.Count != 2 || len(got.Samples) != 2 {
		t.Fatalf("count = %d, samples = %d, want 2/2", got.Count, len(got.Samples))
	}
	first := got.Samples[0]
	if first.StatusCode == nil || *first.StatusCode != 403 {
		t.Errorf("sample 1 statusCode = %v, want 403", first.StatusCode)
	}
	if first.Outcome != "failure" || first.URLPath != "/otp" {
		t.Errorf("sample 1 = %+v, want outcome failure and urlPath /otp", first)
	}
	if first.DurationMs == nil || *first.DurationMs != 900 {
		t.Errorf("sample 1 duration_ms = %v, want 900", first.DurationMs)
	}
	if second := got.Samples[1]; second.StatusCode == nil || *second.StatusCode != 200 {
		t.Errorf("sample 2 statusCode = %v, want 200", second.StatusCode)
	}
}

// Console proxy може бути вимкнений або без прав на індекси — зразки мусять
// повернутися й тоді, лише без доповнених полів.
func TestFetchTransactionSamplesSurvivesProxyFailure(t *testing.T) {
	client := fakeKibana(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/console/proxy" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"no permission"}`))
			return
		}
		_, _ = w.Write([]byte(`{"traceSamples":[{"traceId":"t1","transactionId":"x1"}]}`))
	})

	text, err := FetchTransactionSamples(client, SampleParams{Service: "ms-auth", Transaction: "POST /otp"})
	if err != nil {
		t.Fatalf("enrichment failure must not fail the whole call: %v", err)
	}

	var got samplesResult
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Samples) != 1 || got.Samples[0].TraceID != "t1" {
		t.Errorf("samples = %+v, want the APM sample preserved", got.Samples)
	}
	if !strings.Contains(got.Note, "403") {
		t.Errorf("note = %q, want it to explain why the fields are missing", got.Note)
	}
}

func TestFetchErrorDetailNoHits(t *testing.T) {
	client := fakeKibana(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	})

	text, err := FetchErrorDetail(client, "ms-auth", "missing", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var got errorDetail
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if got.Hint == "" {
		t.Error("expected a hint telling the caller where to get a valid groupId")
	}
}
