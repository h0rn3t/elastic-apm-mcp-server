package kibana

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Client — HTTP-клієнт до внутрішнього Kibana APM API.
type Client struct {
	baseURL    string
	headers    http.Header
	httpClient *http.Client
}

// NewClient створює клієнт з KIBANA_URL, KIBANA_USERNAME, KIBANA_PASSWORD.
func NewClient() (*Client, error) {
	kibanaURL := os.Getenv("KIBANA_URL")
	username := os.Getenv("KIBANA_USERNAME")
	password := os.Getenv("KIBANA_PASSWORD")
	if kibanaURL == "" || username == "" || password == "" {
		return nil, fmt.Errorf("KIBANA_URL, KIBANA_USERNAME, KIBANA_PASSWORD must be set")
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	h := make(http.Header)
	h.Set("Authorization", "Basic "+credentials)
	h.Set("kbn-xsrf", "true")
	h.Set("x-elastic-internal-origin", "kibana")
	h.Set("Content-Type", "application/json")

	return &Client{
		baseURL: strings.TrimRight(kibanaURL, "/"),
		headers: h,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func envOrAll(environment string) string {
	if environment == "" {
		return "ENVIRONMENT_ALL"
	}
	return environment
}

func (c *Client) request(path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header = c.headers.Clone()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Kibana API error %d: %s", resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// GetServices повертає список APM-сервісів. kuery — необов'язковий KQL-фільтр.
func (c *Client) GetServices(start, end, environment, kuery string) (json.RawMessage, error) {
	params := url.Values{
		"start":              {start},
		"end":                {end},
		"environment":        {envOrAll(environment)},
		"probability":        {"1"},
		"documentType":       {"transactionMetric"},
		"rollupInterval":     {"60m"},
		"kuery":              {kuery},
		"useDurationSummary": {"true"},
	}
	var out json.RawMessage
	err := c.request("/internal/apm/services", params, &out)
	return out, err
}

// ServiceOverview — статистика транзакцій і помилок для сервісу.
type ServiceOverview struct {
	Transactions json.RawMessage `json:"transactions"`
	Errors       json.RawMessage `json:"errors"`
}

// GetServiceOverview завантажує транзакції та помилки паралельно.
// kuery — необов'язковий KQL-фільтр, застосовується до обох запитів.
func (c *Client) GetServiceOverview(serviceName, start, end, environment, latencyAggregationType, kuery string) (*ServiceOverview, error) {
	if latencyAggregationType == "" {
		latencyAggregationType = "avg"
	}
	env := envOrAll(environment)
	base := "/internal/apm/services/" + url.PathEscape(serviceName)
	isAvg := latencyAggregationType == "avg"

	txParams := url.Values{
		"start":                  {start},
		"end":                    {end},
		"environment":            {env},
		"transactionType":        {"request"},
		"latencyAggregationType": {latencyAggregationType},
		"kuery":                  {kuery},
		"documentType":           {"transactionEvent"},
		"rollupInterval":         {"60m"},
	}
	if isAvg {
		txParams.Set("documentType", "transactionMetric")
		txParams.Set("useDurationSummary", "true")
	}

	errParams := url.Values{
		"start":       {start},
		"end":         {end},
		"environment": {env},
		"kuery":       {kuery},
	}

	var overview ServiceOverview
	var txErr, errErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		txErr = c.request(base+"/transactions/groups/main_statistics", txParams, &overview.Transactions)
	}()
	go func() {
		defer wg.Done()
		errErr = c.request(base+"/errors/groups/main_statistics", errParams, &overview.Errors)
	}()

	wg.Wait()
	if txErr != nil {
		return nil, txErr
	}
	if errErr != nil {
		return nil, errErr
	}
	return &overview, nil
}

// GetErrors повертає групи помилок сервісу. kuery — необов'язковий KQL-фільтр.
func (c *Client) GetErrors(serviceName, start, end, environment, kuery string) (json.RawMessage, error) {
	params := url.Values{
		"start":       {start},
		"end":         {end},
		"environment": {envOrAll(environment)},
		"kuery":       {kuery},
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/errors/groups/main_statistics"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetServiceMetrics повертає runtime-метрики сервісу. kuery — необов'язковий KQL-фільтр.
func (c *Client) GetServiceMetrics(serviceName, agentName, start, end, environment, serviceRuntimeName, kuery string) (json.RawMessage, error) {
	params := url.Values{
		"start":       {start},
		"end":         {end},
		"agentName":   {agentName},
		"environment": {envOrAll(environment)},
		"kuery":       {kuery},
	}
	if serviceRuntimeName != "" {
		params.Set("serviceRuntimeName", serviceRuntimeName)
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/metrics/charts"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetTransactions повертає статистику транзакцій. kuery — необов'язковий KQL-фільтр.
func (c *Client) GetTransactions(serviceName, start, end, transactionType, environment, latencyAggregationType, kuery string) (json.RawMessage, error) {
	if transactionType == "" {
		transactionType = "request"
	}
	if latencyAggregationType == "" {
		latencyAggregationType = "avg"
	}
	isAvg := latencyAggregationType == "avg"
	params := url.Values{
		"start":                  {start},
		"end":                    {end},
		"transactionType":        {transactionType},
		"environment":            {envOrAll(environment)},
		"latencyAggregationType": {latencyAggregationType},
		"documentType":           {"transactionEvent"},
		"rollupInterval":         {"60m"},
		"kuery":                  {kuery},
	}
	if isAvg {
		params.Set("documentType", "transactionMetric")
		params.Set("useDurationSummary", "true")
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/transactions/groups/main_statistics"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetTransactionSamples повертає зразки трейсів для транзакції.
// kuery — необов'язковий KQL-фільтр, напр. `http.response.status_code : 403`.
func (c *Client) GetTransactionSamples(serviceName, transactionName, start, end, transactionType, environment, sampleRangeFrom, sampleRangeTo, kuery string) (json.RawMessage, error) {
	if transactionType == "" {
		transactionType = "request"
	}
	params := url.Values{
		"start":           {start},
		"end":             {end},
		"transactionType": {transactionType},
		"transactionName": {transactionName},
		"environment":     {envOrAll(environment)},
		"kuery":           {kuery},
	}
	if sampleRangeFrom != "" {
		params.Set("sampleRangeFrom", sampleRangeFrom)
	}
	if sampleRangeTo != "" {
		params.Set("sampleRangeTo", sampleRangeTo)
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/transactions/traces/samples"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetTrace повертає повний waterfall трейсу.
func (c *Client) GetTrace(traceID, start, end, entryTransactionID string) (json.RawMessage, error) {
	params := url.Values{
		"start": {start},
		"end":   {end},
	}
	if entryTransactionID != "" {
		params.Set("entryTransactionId", entryTransactionID)
	}
	path := "/internal/apm/traces/" + url.PathEscape(traceID)
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// Межі size для _search: внутрішній APM API ліміту не має, але відповідь
// повертається в контекст LLM, тому дефолт малий, а максимум обмежений.
const (
	DefaultSearchSize = 10
	MaxSearchSize     = 1000
)

// indexAllowed — символи, допустимі в назві індексу або патерні, включно з
// cross-cluster формою `cluster:index`. Усе інше — насамперед "/", "?", "#" —
// могло б вивести запит за межі `<index>/_search` на інший ендпоінт ES.
const indexAllowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.*,:+"

// SearchRequest — запит до Elasticsearch `_search` через Kibana console proxy.
// Index у тіло не потрапляє: він стає частиною шляху.
type SearchRequest struct {
	Index  string   `json:"-"`
	Query  any      `json:"query,omitempty"`
	Aggs   any      `json:"aggs,omitempty"`
	Source []string `json:"_source,omitempty"`
	Size   int      `json:"size"`
	Sort   any      `json:"sort,omitempty"`
}

func validateIndex(index string) error {
	switch {
	case index == "":
		return errors.New("index must not be empty")
	case len(index) > 255:
		return fmt.Errorf("index must be at most 255 characters, got %d", len(index))
	case strings.Contains(index, ".."):
		return fmt.Errorf("index %q must not contain %q", index, "..")
	}
	if strings.ContainsFunc(index, func(r rune) bool { return !strings.ContainsRune(indexAllowed, r) }) {
		return fmt.Errorf("index %q may contain only letters, digits and -_.*,:+", index)
	}
	return nil
}

// Search виконує `<index>/_search` в Elasticsearch через Kibana console proxy
// (`/api/console/proxy`) — тими самими креденшелами, що й решта клієнта.
// Це escape hatch для полів, яких внутрішній APM API не віддає: error.culprit,
// error.exception[].stacktrace[], довільні _source-поля.
func (c *Client) Search(r SearchRequest) (json.RawMessage, error) {
	if err := validateIndex(r.Index); err != nil {
		return nil, err
	}
	switch {
	case r.Size == 0:
		r.Size = DefaultSearchSize
	case r.Size < 0 || r.Size > MaxSearchSize:
		return nil, fmt.Errorf("size must be between 1 and %d, got %d", MaxSearchSize, r.Size)
	}

	// expand_wildcards=open,hidden: backing індекси data streams (.ds-*) —
	// приховані, і без цього патерн на кшталт ".ds-*apm*" не розкривається ні в що.
	var out json.RawMessage
	err := c.consoleProxy(r.Index+"/_search?expand_wildcards=open,hidden", r, &out)
	return out, err
}

// consoleProxy надсилає POST через Kibana console proxy до Elasticsearch.
// esPath — шлях у ES; він урл-енкодиться як параметр `path`.
func (c *Client) consoleProxy(esPath string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	params := url.Values{
		"path":   {esPath},
		"method": {http.MethodPost},
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/console/proxy?"+params.Encode(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	return c.do(req, out)
}

// GetKibanaVersion повертає версію Kibana через публічний /api/status.
// Потрібна для діагностики підключення: не залежить від APM-індексів.
func (c *Client) GetKibanaVersion() (string, error) {
	var out struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
	}
	if err := c.request("/api/status", nil, &out); err != nil {
		return "", err
	}
	if out.Version.Number == "" {
		return "", errors.New("kibana status has no version.number")
	}
	return out.Version.Number, nil
}
