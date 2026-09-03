package kibana

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateIndex(t *testing.T) {
	tests := []struct {
		name  string
		index string
		ok    bool
	}{
		{"default apm pattern", ".ds-*apm*", true},
		{"comma separated", "traces-apm*,apm-*-transaction*", true},
		{"cross cluster", "prod:logs-apm.error-default", true},
		{"plain name", "logs-apm.error-default-2026.09.03-000001", true},
		{"empty", "", false},
		{"path escape", "logs-apm/_search?pretty=true", false},
		{"other endpoint", "_cluster/settings", false},
		{"traversal", "../_all", false},
		{"query string", "apm*?ignore_unavailable=true", false},
		{"fragment", "apm*#frag", false},
		{"space", "apm* , logs*", false},
		{"newline", "apm*\n_search", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIndex(tt.index)
			if tt.ok && err != nil {
				t.Fatalf("validateIndex(%q) = %v, want nil", tt.index, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("validateIndex(%q) = nil, want error", tt.index)
			}
		})
	}
}

func TestValidateIndexTooLong(t *testing.T) {
	if err := validateIndex(strings.Repeat("a", 256)); err == nil {
		t.Fatal("expected error for a 256-character index")
	}
}

func TestSearchSizeOutOfRange(t *testing.T) {
	c := &Client{baseURL: "http://kibana.invalid", headers: make(http.Header), httpClient: http.DefaultClient}
	for _, size := range []int{-1, MaxSearchSize + 1} {
		if _, err := c.Search(SearchRequest{Index: "apm*", Size: size}); err == nil {
			t.Fatalf("Search(size=%d) = nil error, want error", size)
		}
	}
}

// newTestClient повертає клієнт, який говорить із h замість справжньої Kibana.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{baseURL: srv.URL, headers: make(http.Header), httpClient: srv.Client()}
}

func TestSearchThroughConsoleProxy(t *testing.T) {
	var gotPath, gotMethodParam, gotProxyPath string
	var gotBody map[string]any

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotProxyPath = r.URL.Query().Get("path")
		gotMethodParam = r.URL.Query().Get("method")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	})

	raw, err := c.Search(SearchRequest{
		Index: "logs-apm.error*",
		Query: map[string]any{"term": map[string]any{"service.name": "ms-auth"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/console/proxy" {
		t.Errorf("path = %q, want /api/console/proxy", gotPath)
	}
	// Шлях мусить нести і індекс, і expand_wildcards — інакше приховані
	// backing індекси data streams не розкриваються.
	if gotProxyPath != "logs-apm.error*/_search?expand_wildcards=open,hidden" {
		t.Errorf("path param = %q, want the index, _search and expand_wildcards", gotProxyPath)
	}
	if gotMethodParam != http.MethodPost {
		t.Errorf("method param = %q, want POST", gotMethodParam)
	}
	if size, _ := gotBody["size"].(float64); int(size) != DefaultSearchSize {
		t.Errorf("body size = %v, want %d", gotBody["size"], DefaultSearchSize)
	}
	if _, ok := gotBody["query"]; !ok {
		t.Error("body has no query")
	}
	if _, ok := gotBody["index"]; ok {
		t.Error("index must not appear in the request body")
	}
	if _, ok := gotBody["sort"]; ok {
		t.Error("unset sort must be omitted from the body")
	}
	if string(raw) != `{"hits":{"hits":[]}}` {
		t.Errorf("raw = %s, want the response passed through unchanged", raw)
	}
}

func TestSearchErrorStatus(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})
	_, err := c.Search(SearchRequest{Index: "apm*"})
	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want the status code in the message", err)
	}
}

func TestKueryReachesTheQueryString(t *testing.T) {
	var got string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("kuery")
		_, _ = w.Write([]byte(`{}`))
	})
	const kuery = `http.response.status_code : 403`
	if _, err := c.GetTransactionSamples("ms-auth", "POST /otp", "s", "e", "", "", "", "", kuery); err != nil {
		t.Fatal(err)
	}
	if got != kuery {
		t.Errorf("kuery = %q, want %q", got, kuery)
	}
}
