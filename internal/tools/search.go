package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DefaultSearchIndex покриває APM data streams 8.x (.ds-traces-apm-*,
// .ds-logs-apm.error-*, .ds-metrics-apm-*) одним патерном.
const DefaultSearchIndex = ".ds-*apm*"

const searchToolDescription = `Escape hatch to Elasticsearch: run a raw _search against the APM indices through the Kibana console proxy (same credentials as every other tool). Use for what the internal APM API never returns — error.culprit, error.exception[].stacktrace[], arbitrary _source fields, exact term filters on any field. Prefer the dedicated tools when they answer the question; reach here instead of shelling out to curl. Read-only: the path is always <index>/_search, so no aggregations-only endpoints, no writes.`

func registerSearchTools(s *server.MCPServer, client *kibana.Client) {
	tool := mcp.NewTool("search_raw",
		mcp.WithDescription(searchToolDescription),
		mcp.WithObject("query",
			mcp.Description(`Elasticsearch query DSL, e.g. {"bool":{"filter":[{"term":{"service.name":"ms-auth"}},{"range":{"@timestamp":{"gte":"now-1h"}}}]}}. Omit to match everything`),
		),
		mcp.WithString("index",
			mcp.Description(`Index or index pattern. Default `+DefaultSearchIndex+`. Narrow it for speed: traces-apm* (transactions/spans), logs-apm.error* (errors), metrics-apm*`),
		),
		mcp.WithArray("_source",
			mcp.WithStringItems(),
			mcp.Description(`Fields to return, e.g. ["@timestamp","error.culprit","error.exception.stacktrace"]. Omit for the whole document — usually far more than needed`),
		),
		mcp.WithNumber("size",
			mcp.Description(fmt.Sprintf("Number of hits (1–%d, default %d). Every hit lands in the model context, so keep it small and trim with _source", kibana.MaxSearchSize, kibana.DefaultSearchSize)),
		),
		mcp.WithArray("sort",
			mcp.Description(`Sort clauses, e.g. [{"@timestamp":{"order":"desc"}}]`),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		raw, err := client.Search(kibana.SearchRequest{
			Index:  req.GetString("index", DefaultSearchIndex),
			Query:  args["query"],
			Source: req.GetStringSlice("_source", nil),
			Size:   req.GetInt("size", 0),
			Sort:   args["sort"],
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search raw: %v", err)), nil
		}
		text, err := formatRawJSON(raw)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	})
}

// searchHits дістає _source кожного хіта з відповіді Elasticsearch.
func searchHits(raw json.RawMessage) []map[string]any {
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	hits := toMapSlice(anySlice(obj(resp["hits"])["hits"]))
	out := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		if src := obj(hit["_source"]); src != nil {
			out = append(out, src)
		}
	}
	return out
}
