package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildErrorDetail(t *testing.T) {
	const doc = `{
		"@timestamp": "2026-09-03T10:00:00Z",
		"error": {
			"grouping_key": "abc123",
			"culprit": "otp.Verify (otp.go)",
			"exception": [
				{
					"type": "AuthError",
					"message": "otp expired",
					"handled": false,
					"stacktrace": [
						{"function": "Verify", "filename": "otp.go", "line": {"number": 42}},
						{"function": "ServeHTTP", "abs_path": "/src/http.go", "line": {"number": 17}}
					]
				},
				{"type": "SQLError", "message": "no rows"}
			]
		},
		"transaction": {"id": "x1", "name": "POST /ms-auth/api/v1/jwt/otp"},
		"trace": {"id": "t1"},
		"url": {"full": "https://host/ms-auth/api/v1/jwt/otp"},
		"http": {"response": {"status_code": 403}}
	}`
	var src map[string]any
	if err := json.Unmarshal([]byte(doc), &src); err != nil {
		t.Fatal(err)
	}

	got := buildErrorDetail("ms-auth", "abc123", src)

	if got.Culprit != "otp.Verify (otp.go)" {
		t.Errorf("culprit = %q", got.Culprit)
	}
	if len(got.Exceptions) != 2 {
		t.Fatalf("got %d exceptions, want the whole chain (2)", len(got.Exceptions))
	}
	if got.Exceptions[0].Handled == nil || *got.Exceptions[0].Handled {
		t.Errorf("handled = %v, want false", got.Exceptions[0].Handled)
	}
	want := []string{"at Verify (otp.go:42)", "at ServeHTTP (/src/http.go:17)"}
	for i, frame := range want {
		if got.Exceptions[0].Stacktrace[i] != frame {
			t.Errorf("frame %d = %q, want %q", i, got.Exceptions[0].Stacktrace[i], frame)
		}
	}
	if got.TransactionName != "POST /ms-auth/api/v1/jwt/otp" || got.TraceID != "t1" {
		t.Errorf("transaction context = %q/%q", got.TransactionName, got.TraceID)
	}
	if got.StatusCode == nil || *got.StatusCode != 403 {
		t.Errorf("statusCode = %v, want 403", got.StatusCode)
	}
	if got.Hint != "" {
		t.Errorf("hint = %q, want none for a document with a culprit", got.Hint)
	}
}

func TestBuildErrorDetailLogOnly(t *testing.T) {
	var src map[string]any
	err := json.Unmarshal([]byte(`{"error":{"log":{"message":"db unreachable","stacktrace":[{"function":"Connect"}]}}}`), &src)
	if err != nil {
		t.Fatal(err)
	}
	got := buildErrorDetail("ms-auth", "abc123", src)
	if got.LogMessage != "db unreachable" {
		t.Errorf("logMessage = %q", got.LogMessage)
	}
	if len(got.LogStacktrace) != 1 || got.LogStacktrace[0] != "at Connect" {
		t.Errorf("logStacktrace = %v, want [at Connect]", got.LogStacktrace)
	}
}

func TestBuildErrorDetailEmptyDocumentHints(t *testing.T) {
	got := buildErrorDetail("ms-auth", "abc123", map[string]any{})
	if got.Hint == "" {
		t.Error("expected a hint when the document carried no culprit, exception or log message")
	}
	if got.Service != "ms-auth" || got.GroupID != "abc123" {
		t.Errorf("identity fields = %q/%q", got.Service, got.GroupID)
	}
}

func TestFormatStacktraceTruncates(t *testing.T) {
	frames := make([]any, maxStackFrames+7)
	for i := range frames {
		frames[i] = map[string]any{"function": "Frame"}
	}

	got := formatStacktrace(frames)
	if len(got) != maxStackFrames+1 {
		t.Fatalf("got %d lines, want %d frames plus one summary line", len(got), maxStackFrames)
	}
	if !strings.Contains(got[maxStackFrames], "7 more frames") {
		t.Errorf("last line = %q, want the count of dropped frames", got[maxStackFrames])
	}
	if formatStacktrace(nil) != nil {
		t.Error("formatStacktrace(nil) must be nil, not an empty slice")
	}
}

func TestFormatStacktraceSkipsEmptyFrames(t *testing.T) {
	got := formatStacktrace([]any{
		map[string]any{},
		map[string]any{"filename": "otp.go", "line": map[string]any{"number": 9.0}},
		map[string]any{"function": "Verify"},
	})
	want := []string{"at otp.go:9", "at Verify"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
