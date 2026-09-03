package tools

import (
	"encoding/json"
	"sort"
)

type traceNode struct {
	Kind        string
	ID          string
	ParentID    string
	Service     string
	Name        string
	Type        string
	Subtype     string
	Resource    string
	DurationUs  int64
	TimestampUs int64
	Children    []*traceNode
	SelfUs      int64
}

func parseDoc(doc map[string]any) *traceNode {
	evt, _ := doc["processor"].(map[string]any)["event"].(string)
	switch evt {
	case "transaction":
		tx, _ := doc["transaction"].(map[string]any)
		id, _ := tx["id"].(string)
		if id == "" {
			return nil
		}
		svc, _ := doc["service"].(map[string]any)
		parent, _ := doc["parent"].(map[string]any)
		return &traceNode{
			Kind:        "transaction",
			ID:          id,
			ParentID:    str(parent["id"]),
			Service:     str(svc["name"]),
			Name:        str(tx["name"]),
			Type:        str(tx["type"]),
			DurationUs:  durationUs(tx["duration"]),
			TimestampUs: timestampUs(doc["timestamp"]),
		}
	case "span":
		span, _ := doc["span"].(map[string]any)
		id, _ := span["id"].(string)
		if id == "" {
			return nil
		}
		svc, _ := doc["service"].(map[string]any)
		parent, _ := doc["parent"].(map[string]any)
		dest, _ := span["destination"].(map[string]any)
		svcDest, _ := dest["service"].(map[string]any)
		return &traceNode{
			Kind:        "span",
			ID:          id,
			ParentID:    str(parent["id"]),
			Service:     str(svc["name"]),
			Name:        str(span["name"]),
			Type:        str(span["type"]),
			Subtype:     str(span["subtype"]),
			Resource:    str(svcDest["resource"]),
			DurationUs:  durationUs(span["duration"]),
			TimestampUs: timestampUs(doc["timestamp"]),
		}
	}
	return nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func durationUs(v any) int64 {
	if m, ok := v.(map[string]any); ok {
		switch us := m["us"].(type) {
		case float64:
			return int64(us)
		case int:
			return int64(us)
		case int64:
			return us
		}
	}
	return 0
}

func timestampUs(v any) int64 {
	if m, ok := v.(map[string]any); ok {
		switch us := m["us"].(type) {
		case float64:
			return int64(us)
		case int:
			return int64(us)
		case int64:
			return us
		}
	}
	return 0
}

func toMs(us int64) int64 {
	return (us + 500) / 1000
}

func pct(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(int64(float64(part)/float64(whole)*1000+0.5)) / 10
}

type renderedNode struct {
	Kind      string         `json:"kind"`
	Service   string         `json:"service"`
	Name      string         `json:"name"`
	Duration  int64          `json:"duration_ms"`
	Self      int64          `json:"self_ms"`
	PctOfRoot float64        `json:"pctOfRoot"`
	Type      string         `json:"type,omitempty"`
	Subtype   string         `json:"subtype,omitempty"`
	Resource  string         `json:"resource,omitempty"`
	Children  []renderedNode `json:"children,omitempty"`
}

func renderNode(n *traceNode, rootUs int64) renderedNode {
	out := renderedNode{
		Kind:      n.Kind,
		Service:   n.Service,
		Name:      n.Name,
		Duration:  toMs(n.DurationUs),
		Self:      toMs(n.SelfUs),
		PctOfRoot: pct(n.DurationUs, rootUs),
		Type:      n.Type,
		Subtype:   n.Subtype,
		Resource:  n.Resource,
	}
	if len(n.Children) > 0 {
		out.Children = make([]renderedNode, len(n.Children))
		for i, c := range n.Children {
			out.Children[i] = renderNode(c, rootUs)
		}
	}
	return out
}

func flatten(n *traceNode, acc []*traceNode) []*traceNode {
	acc = append(acc, n)
	for _, c := range n.Children {
		acc = flatten(c, acc)
	}
	return acc
}

// SummarizeTrace будує waterfall і top bottlenecks з відповіді Kibana.
func SummarizeTrace(raw json.RawMessage) (json.RawMessage, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	traceItems, _ := root["traceItems"].(map[string]any)
	docs := toMapSlice(anySlice(traceItems["traceDocs"]))
	errorDocs := anySlice(traceItems["errorDocs"])

	nodes := make(map[string]*traceNode)
	for _, doc := range docs {
		if n := parseDoc(doc); n != nil {
			nodes[n.ID] = n
		}
	}

	var roots []*traceNode
	for _, n := range nodes {
		if n.ParentID != "" {
			if parent, ok := nodes[n.ParentID]; ok {
				parent.Children = append(parent.Children, n)
				continue
			}
		}
		roots = append(roots, n)
	}

	var finalize func(*traceNode)
	finalize = func(n *traceNode) {
		sort.Slice(n.Children, func(i, j int) bool {
			return n.Children[i].DurationUs > n.Children[j].DurationUs
		})
		for _, c := range n.Children {
			finalize(c)
		}
		var childSum int64
		for _, c := range n.Children {
			childSum += c.DurationUs
		}
		n.SelfUs = max(n.DurationUs-childSum, 0)
	}
	for _, r := range roots {
		finalize(r)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].DurationUs > roots[j].DurationUs
	})

	var rootUs int64
	if len(roots) > 0 {
		rootUs = roots[0].DurationUs
	}

	var allNodes []*traceNode
	for _, r := range roots {
		allNodes = flatten(r, allNodes)
	}

	type bottleneck struct {
		Kind      string  `json:"kind"`
		Service   string  `json:"service"`
		Name      string  `json:"name"`
		Type      string  `json:"type,omitempty"`
		Subtype   string  `json:"subtype,omitempty"`
		Resource  string  `json:"resource,omitempty"`
		SelfMs    int64   `json:"self_ms"`
		Duration  int64   `json:"duration_ms"`
		PctOfRoot float64 `json:"pctOfRoot"`
	}

	sorted := append([]*traceNode(nil), allNodes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SelfUs > sorted[j].SelfUs
	})
	top := make([]bottleneck, 0, 5)
	for i := 0; i < len(sorted) && i < 5; i++ {
		n := sorted[i]
		top = append(top, bottleneck{
			Kind:      n.Kind,
			Service:   n.Service,
			Name:      n.Name,
			Type:      n.Type,
			Subtype:   n.Subtype,
			Resource:  n.Resource,
			SelfMs:    toMs(n.SelfUs),
			Duration:  toMs(n.DurationUs),
			PctOfRoot: pct(n.DurationUs, rootUs),
		})
	}

	waterfall := make([]renderedNode, len(roots))
	for i, r := range roots {
		waterfall[i] = renderNode(r, rootUs)
	}

	totalItems := len(allNodes)
	if v, ok := traceItems["traceDocsTotal"].(float64); ok {
		totalItems = int(v)
	}

	exceedsMax, _ := traceItems["exceedsMax"].(bool)

	result := map[string]any{
		"summary": map[string]any{
			"rootDuration_ms": toMs(rootUs),
			"totalItems":      totalItems,
			"errorCount":      len(errorDocs),
			"exceedsMax":      exceedsMax,
			"topBottlenecks":  top,
		},
		"waterfall": waterfall,
	}

	if entry, ok := root["entryTransaction"].(map[string]any); ok {
		tx, _ := entry["transaction"].(map[string]any)
		svc, _ := entry["service"].(map[string]any)
		urlMap, _ := entry["url"].(map[string]any)
		result["entryTransaction"] = map[string]any{
			"service":     str(svc["name"]),
			"name":        str(tx["name"]),
			"duration_ms": toMs(durationUs(tx["duration"])),
			"timestamp":   entry["@timestamp"],
			"url":         str(urlMap["full"]),
			"http":        entry["http"],
		}
	}

	return json.MarshalIndent(result, "", "  ")
}
