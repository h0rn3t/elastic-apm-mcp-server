package kibana

import (
	"encoding/base64"
	"encoding/json"
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

// GetServices повертає список APM-сервісів.
func (c *Client) GetServices(start, end, environment string) (json.RawMessage, error) {
	params := url.Values{
		"start":              {start},
		"end":                {end},
		"environment":        {envOrAll(environment)},
		"probability":        {"1"},
		"documentType":       {"transactionMetric"},
		"rollupInterval":     {"60m"},
		"kuery":              {""},
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
func (c *Client) GetServiceOverview(serviceName, start, end, environment, latencyAggregationType string) (*ServiceOverview, error) {
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
		"kuery":                  {""},
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
		"kuery":       {""},
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

// GetErrors повертає групи помилок сервісу.
func (c *Client) GetErrors(serviceName, start, end, environment string) (json.RawMessage, error) {
	params := url.Values{
		"start":       {start},
		"end":         {end},
		"environment": {envOrAll(environment)},
		"kuery":       {""},
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/errors/groups/main_statistics"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetServiceMetrics повертає runtime-метрики сервісу.
func (c *Client) GetServiceMetrics(serviceName, agentName, start, end, environment, serviceRuntimeName string) (json.RawMessage, error) {
	params := url.Values{
		"start":       {start},
		"end":         {end},
		"agentName":   {agentName},
		"environment": {envOrAll(environment)},
		"kuery":       {""},
	}
	if serviceRuntimeName != "" {
		params.Set("serviceRuntimeName", serviceRuntimeName)
	}
	path := "/internal/apm/services/" + url.PathEscape(serviceName) + "/metrics/charts"
	var out json.RawMessage
	err := c.request(path, params, &out)
	return out, err
}

// GetTransactions повертає статистику транзакцій.
func (c *Client) GetTransactions(serviceName, start, end, transactionType, environment, latencyAggregationType string) (json.RawMessage, error) {
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
		"kuery":                  {""},
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
func (c *Client) GetTransactionSamples(serviceName, transactionName, start, end, transactionType, environment, sampleRangeFrom, sampleRangeTo string) (json.RawMessage, error) {
	if transactionType == "" {
		transactionType = "request"
	}
	params := url.Values{
		"start":           {start},
		"end":             {end},
		"transactionType": {transactionType},
		"transactionName": {transactionName},
		"environment":     {envOrAll(environment)},
		"kuery":           {""},
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
