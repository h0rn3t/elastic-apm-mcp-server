package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
)

func newVersionClient(t *testing.T, status int, body string) *kibana.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("KIBANA_URL", srv.URL)
	t.Setenv("KIBANA_USERNAME", "u")
	t.Setenv("KIBANA_PASSWORD", "p")
	client, err := kibana.NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestFetchVersionSuccess(t *testing.T) {
	client := newVersionClient(t, http.StatusOK, `{"version":{"number":"8.17.3"}}`)

	text, err := FetchVersion(client, "0.2.0")
	if err != nil {
		t.Fatalf("FetchVersion() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("FetchVersion() returned invalid JSON: %v", err)
	}
	if got["binaryVersion"] != "0.2.0" {
		t.Errorf("binaryVersion = %v, want 0.2.0", got["binaryVersion"])
	}
	if got["kibanaVersion"] != "8.17.3" {
		t.Errorf("kibanaVersion = %v, want 8.17.3", got["kibanaVersion"])
	}
	if _, ok := got["error"]; ok {
		t.Errorf("error = %v, want no error field on success", got["error"])
	}
}

func TestFetchVersionUnreachable(t *testing.T) {
	client := newVersionClient(t, http.StatusUnauthorized, `{"error":"unauthorized"}`)

	text, err := FetchVersion(client, "0.2.0")
	if err != nil {
		t.Fatalf("FetchVersion() error = %v, want the failure embedded in JSON", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("FetchVersion() returned invalid JSON: %v", err)
	}
	if got["kibana"] != "unreachable" {
		t.Errorf("kibana = %v, want unreachable", got["kibana"])
	}
	msg, _ := got["error"].(string)
	if !strings.Contains(msg, "401") {
		t.Errorf("error = %q, want the status code in the message", msg)
	}
}
