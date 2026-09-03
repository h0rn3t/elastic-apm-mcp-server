package tools

import (
	"context"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// errorIndex — error-документи в data streams 8.x і в legacy-індексах 7.x.
const errorIndex = "logs-apm.error*,apm-*-error*"

// maxStackFrames обмежує один stacktrace: у JVM/Spring їх бувають сотні, а
// найінформативніші — верхні кадри.
const maxStackFrames = 50

const errorDetailToolDescription = `Full detail of one error group: culprit, the exception chain with stacktraces, the transaction it happened in, and the request URL. This is what get_errors cannot show — the internal APM API never returns error.culprit or error.exception[].stacktrace[]. Read straight from the Elasticsearch error document (via the Kibana console proxy), so it does not depend on the Kibana version. Get groupId from the errorGroups[].groupId field of get_errors.`

type exceptionDetail struct {
	Type       string   `json:"type,omitempty"`
	Message    string   `json:"message,omitempty"`
	Module     string   `json:"module,omitempty"`
	Handled    *bool    `json:"handled,omitempty"`
	Stacktrace []string `json:"stacktrace,omitempty"`
}

type errorDetail struct {
	Service         string            `json:"service"`
	GroupID         string            `json:"groupId"`
	Timestamp       string            `json:"timestamp,omitempty"`
	Culprit         string            `json:"culprit,omitempty"`
	LogMessage      string            `json:"logMessage,omitempty"`
	Exceptions      []exceptionDetail `json:"exceptions,omitempty"`
	LogStacktrace   []string          `json:"logStacktrace,omitempty"`
	TransactionName string            `json:"transactionName,omitempty"`
	TransactionID   string            `json:"transactionId,omitempty"`
	TraceID         string            `json:"traceId,omitempty"`
	URL             string            `json:"url,omitempty"`
	StatusCode      *int              `json:"statusCode,omitempty"`
	Hint            string            `json:"hint,omitempty"`
}

func registerErrorDetailTool(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("get_error_detail",
		mcp.WithDescription(errorDetailToolDescription),
		mcp.WithString("serviceName",
			mcp.Description("APM service name, e.g. ms-auth"),
			mcp.Required(),
		),
		mcp.WithString("groupId",
			mcp.Description("Error group id (error.grouping_key) — the groupId field of a get_errors error group"),
			mcp.Required(),
		),
		mcp.WithString("start",
			mcp.Description("Range start (ISO 8601). Optional — omit to take the latest occurrence ever recorded"),
		),
		mcp.WithString("end",
			mcp.Description("Range end (ISO 8601)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serviceName := req.GetString("serviceName", "")
		groupID := req.GetString("groupId", "")
		if serviceName == "" || groupID == "" {
			return mcp.NewToolResultError("serviceName and groupId are required"), nil
		}
		text, err := FetchErrorDetail(client, serviceName, groupID, req.GetString("start", ""), req.GetString("end", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get error detail: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}

// FetchErrorDetail повертає найсвіжіший error-документ групи, зведений для
// читання. start/end необов'язкові; порожні означають «за весь час».
func FetchErrorDetail(client *kibana.Client, serviceName, groupID, start, end string) (string, error) {
	filters := []any{
		map[string]any{"term": map[string]any{"service.name": serviceName}},
		map[string]any{"term": map[string]any{"error.grouping_key": groupID}},
	}
	if start != "" || end != "" {
		bounds := make(map[string]any, 2)
		if start != "" {
			bounds["gte"] = start
		}
		if end != "" {
			bounds["lte"] = end
		}
		filters = append(filters, map[string]any{"range": map[string]any{"@timestamp": bounds}})
	}

	raw, err := client.Search(kibana.SearchRequest{
		Index: errorIndex,
		Size:  1,
		Query: map[string]any{"bool": map[string]any{"filter": filters}},
		Sort:  []any{map[string]any{"@timestamp": map[string]any{"order": "desc"}}},
	})
	if err != nil {
		return "", err
	}

	hits := searchHits(raw)
	if len(hits) == 0 {
		return formatJSON(errorDetail{
			Service: serviceName,
			GroupID: groupID,
			Hint:    "No error document matched. Check groupId against errorGroups[].groupId from get_errors, confirm the service name, or widen start/end.",
		})
	}
	return formatJSON(buildErrorDetail(serviceName, groupID, hits[0]))
}

func buildErrorDetail(serviceName, groupID string, src map[string]any) errorDetail {
	errDoc := obj(src["error"])
	tx := obj(src["transaction"])
	logDoc := obj(errDoc["log"])

	detail := errorDetail{
		Service:         serviceName,
		GroupID:         groupID,
		Timestamp:       str(src["@timestamp"]),
		Culprit:         str(errDoc["culprit"]),
		LogMessage:      str(logDoc["message"]),
		LogStacktrace:   formatStacktrace(anySlice(logDoc["stacktrace"])),
		TransactionName: str(tx["name"]),
		TransactionID:   str(tx["id"]),
		TraceID:         str(obj(src["trace"])["id"]),
		URL:             pickString(obj(src["url"])["full"], obj(src["url"])["path"]),
		StatusCode:      pickNumber(obj(obj(src["http"])["response"])["status_code"]),
	}

	for _, e := range anySlice(errDoc["exception"]) {
		exc := obj(e)
		item := exceptionDetail{
			Type:       str(exc["type"]),
			Message:    str(exc["message"]),
			Module:     str(exc["module"]),
			Stacktrace: formatStacktrace(anySlice(exc["stacktrace"])),
		}
		if handled, ok := exc["handled"].(bool); ok {
			item.Handled = &handled
		}
		detail.Exceptions = append(detail.Exceptions, item)
	}

	if detail.Culprit == "" && len(detail.Exceptions) == 0 && detail.LogMessage == "" {
		detail.Hint = "The document carried no culprit, exception or log message. Use search_raw on " + errorIndex + " to inspect it in full."
	}
	return detail
}

// formatStacktrace згортає кадри APM у рядки "at func (file:line)".
func formatStacktrace(frames []any) []string {
	if len(frames) == 0 {
		return nil
	}
	out := make([]string, 0, min(len(frames), maxStackFrames+1))
	for _, f := range frames[:min(len(frames), maxStackFrames)] {
		frame := obj(f)
		where := pickString(frame["filename"], frame["abs_path"], frame["module"])
		if line := pickNumber(obj(frame["line"])["number"]); line != nil && where != "" {
			where = fmt.Sprintf("%s:%d", where, *line)
		}
		fn := str(frame["function"])
		switch {
		case fn != "" && where != "":
			out = append(out, fmt.Sprintf("at %s (%s)", fn, where))
		case fn != "":
			out = append(out, "at "+fn)
		case where != "":
			out = append(out, "at "+where)
		}
	}
	if len(frames) > maxStackFrames {
		out = append(out, fmt.Sprintf("... %d more frames (use search_raw for the full stacktrace)", len(frames)-maxStackFrames))
	}
	return out
}
