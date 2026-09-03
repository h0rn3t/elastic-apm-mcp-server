// Package cli експонує операції Elastic APM як підкоманди бінарника.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/h0rn3t/elastic-apm-mcp-server/internal/kibana"
	"github.com/h0rn3t/elastic-apm-mcp-server/internal/tools"
)

// Run виконує CLI-підкоманду. Повертає код виходу процесу.
func Run(args []string, client *kibana.Client) int {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return 0
	case "list-services":
		return cmdListServices(client, args[1:])
	case "get-service-overview":
		return cmdGetServiceOverview(client, args[1:])
	case "get-transactions":
		return cmdGetTransactions(client, args[1:])
	case "get-errors", "check-service-errors":
		return cmdGetErrors(client, args[1:])
	case "get-service-metrics":
		return cmdGetServiceMetrics(client, args[1:])
	case "get-transaction-samples":
		return cmdGetTransactionSamples(client, args[1:])
	case "get-trace":
		return cmdGetTrace(client, args[1:])
	case "get-error-detail":
		return cmdGetErrorDetail(client, args[1:])
	case "search-raw":
		return cmdSearchRaw(client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "невідома команда: %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Використання: elastic-apm-mcp-server <команда> [прапорці]

Читання:
  list-services              Список APM-сервісів
  get-service-overview       Огляд сервісу (транзакції + помилки)
  get-transactions           Статистика транзакцій
  get-errors                 Перевірка помилок (alias: check-service-errors)
  get-error-detail           Culprit, exception-ланцюжок і stacktrace однієї групи
  get-service-metrics        Runtime-метрики (CPU, пам'ять)
  get-transaction-samples    Зразки трейсів для транзакції (зі status_code/outcome/url)
  get-trace                  Waterfall трейсу за trace ID
  search-raw                 Сирий _search в Elasticsearch (escape hatch)

Сервер:
  mcp                        MCP-сервер через stdio (також за замовчуванням)

Часовий діапазон (де застосовно):
  --start, --end             ISO 8601 (обов'язкові для більшості команд)
  --period                   15m, 1h, 6h, 24h, 7d (лише get-errors)
  --hours                    1–168 (лише get-errors, замість period)
  --environment              Фільтр середовища
  --kuery                    KQL-фільтр, напр. 'http.response.status_code : 403'

Деталі: elastic-apm-mcp-server <команда> --help

Змінні середовища:
  KIBANA_URL                 URL Kibana (обов'язково)
  KIBANA_USERNAME            Ім'я користувача (обов'язково)
  KIBANA_PASSWORD            Пароль (обов'язково)
`)
}

func failf(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func parseFlagsOnly(fs *flag.FlagSet, args []string) (int, bool) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Використання: elastic-apm-mcp-server %s [прапорці]\n", fs.Name())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "%s: зайві аргументи: %v\n", fs.Name(), fs.Args())
		return 2, false
	}
	return 0, true
}

func addTimeFlags(fs *flag.FlagSet) (start, end, period, environment *string, hours *float64) {
	start = fs.String("start", "", "Початок діапазону (ISO 8601)")
	end = fs.String("end", "", "Кінець діапазону (ISO 8601)")
	period = fs.String("period", "", "Вікно: 15m, 1h, 6h, 24h, 7d")
	environment = fs.String("environment", "", "Середовище (production, staging, …)")
	hours = fs.Float64("hours", 0, "Інтервал у годинах (1–168), для get-errors")
	return
}

func addKueryFlag(fs *flag.FlagSet) *string {
	return fs.String("kuery", "", "KQL-фільтр, напр. 'http.response.status_code : 403'")
}

func requireStartEnd(start, end string) bool {
	if start == "" || end == "" {
		fmt.Fprintln(os.Stderr, "потрібні --start та --end (ISO 8601)")
		return false
	}
	return true
}

func cmdListServices(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("list-services", flag.ContinueOnError)
	start, end, _, environment, _ := addTimeFlags(fs)
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if !requireStartEnd(*start, *end) {
		return 2
	}
	data, err := client.GetServices(*start, *end, *environment, *kuery)
	if err != nil {
		return failf("list services: %v", err)
	}
	text, err := tools.FormatRawJSON(data)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetServiceOverview(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-service-overview", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	start, end, _, environment, _ := addTimeFlags(fs)
	latency := fs.String("latency", "avg", "Агрегація затримки: avg, p95, p99")
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" || !requireStartEnd(*start, *end) {
		return 2
	}
	data, err := client.GetServiceOverview(*service, *start, *end, *environment, *latency, *kuery)
	if err != nil {
		return failf("service overview: %v", err)
	}
	text, err := tools.FormatJSON(data)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetTransactions(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-transactions", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	start, end, _, environment, _ := addTimeFlags(fs)
	txType := fs.String("transaction-type", "request", "Тип транзакції")
	latency := fs.String("latency", "avg", "Агрегація затримки: avg, p95, p99")
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" || !requireStartEnd(*start, *end) {
		return 2
	}
	data, err := client.GetTransactions(*service, *start, *end, *txType, *environment, *latency, *kuery)
	if err != nil {
		return failf("get transactions: %v", err)
	}
	text, err := tools.FormatRawJSON(data)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetErrors(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-errors", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	start, end, period, environment, hours := addTimeFlags(fs)
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" {
		fmt.Fprintln(os.Stderr, "потрібен --service")
		return 2
	}
	timeArgs, err := tools.ReadTimeArgsCLI(*start, *end, *period, *hours, *hours > 0)
	if err != nil {
		return failf("%v", err)
	}
	timeArgs.Environment = *environment
	timeArgs.Kuery = *kuery
	text, err := tools.FetchServiceErrorsCLI(client, *service, timeArgs)
	if err != nil {
		return failf("get errors: %v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetServiceMetrics(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-service-metrics", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	agent := fs.String("agent", "", "Тип агента: java, nodejs, go, … (обов'язково)")
	start, end, _, environment, _ := addTimeFlags(fs)
	runtime := fs.String("runtime", "", "serviceRuntimeName (деякі версії Kibana)")
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" || *agent == "" || !requireStartEnd(*start, *end) {
		return 2
	}
	data, err := client.GetServiceMetrics(*service, *agent, *start, *end, *environment, *runtime, *kuery)
	if err != nil {
		return failf("get metrics: %v", err)
	}
	text, err := tools.FormatRawJSON(data)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetTransactionSamples(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-transaction-samples", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	transaction := fs.String("transaction", "", "Назва транзакції (обов'язково)")
	start, end, _, environment, _ := addTimeFlags(fs)
	txType := fs.String("transaction-type", "request", "Тип транзакції")
	sampleFrom := fs.String("sample-from", "", "Нижня межа затримки зразка (мкс)")
	sampleTo := fs.String("sample-to", "", "Верхня межа затримки зразка (мкс)")
	kuery := addKueryFlag(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" || *transaction == "" || !requireStartEnd(*start, *end) {
		return 2
	}
	text, err := tools.FetchTransactionSamples(client, tools.SampleParams{
		Service:         *service,
		Transaction:     *transaction,
		Start:           *start,
		End:             *end,
		TransactionType: *txType,
		Environment:     *environment,
		Kuery:           *kuery,
		SampleRangeFrom: *sampleFrom,
		SampleRangeTo:   *sampleTo,
	})
	if err != nil {
		return failf("get transaction samples: %v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetTrace(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-trace", flag.ContinueOnError)
	traceID := fs.String("trace-id", "", "Trace ID (обов'язково)")
	start, end, _, _, _ := addTimeFlags(fs)
	entryTx := fs.String("entry-transaction-id", "", "Entry transaction ID (деякі версії Kibana)")
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *traceID == "" || !requireStartEnd(*start, *end) {
		return 2
	}
	raw, err := client.GetTrace(*traceID, *start, *end, *entryTx)
	if err != nil {
		return failf("get trace: %v", err)
	}
	text, err := tools.SummarizeTraceText(raw)
	if err != nil {
		return failf("summarize trace: %v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdGetErrorDetail(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("get-error-detail", flag.ContinueOnError)
	service := fs.String("service", "", "Назва APM-сервісу (обов'язково)")
	groupID := fs.String("group-id", "", "error.grouping_key групи помилок (обов'язково)")
	start, end, _, _, _ := addTimeFlags(fs)
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}
	if *service == "" || *groupID == "" {
		fmt.Fprintln(os.Stderr, "потрібні --service та --group-id")
		return 2
	}
	text, err := tools.FetchErrorDetail(client, *service, *groupID, *start, *end)
	if err != nil {
		return failf("get error detail: %v", err)
	}
	fmt.Print(text)
	return 0
}

func cmdSearchRaw(client *kibana.Client, args []string) int {
	fs := flag.NewFlagSet("search-raw", flag.ContinueOnError)
	index := fs.String("index", tools.DefaultSearchIndex, "Індекс або патерн")
	query := fs.String("query", "", `Query DSL, напр. '{"term":{"service.name":"ms-auth"}}'`)
	source := fs.String("source", "", "Поля _source через кому")
	sortBy := fs.String("sort", "", `Сортування, напр. '[{"@timestamp":{"order":"desc"}}]'`)
	size := fs.Int("size", kibana.DefaultSearchSize, fmt.Sprintf("Кількість хітів (1–%d)", kibana.MaxSearchSize))
	if code, ok := parseFlagsOnly(fs, args); !ok {
		return code
	}

	req := kibana.SearchRequest{Index: *index, Size: *size}
	if *source != "" {
		req.Source = strings.Split(*source, ",")
	}
	if err := unmarshalFlag("query", *query, &req.Query); err != nil {
		return failf("%v", err)
	}
	if err := unmarshalFlag("sort", *sortBy, &req.Sort); err != nil {
		return failf("%v", err)
	}

	raw, err := client.Search(req)
	if err != nil {
		return failf("search raw: %v", err)
	}
	text, err := tools.FormatRawJSON(raw)
	if err != nil {
		return failf("%v", err)
	}
	fmt.Print(text)
	return 0
}

// unmarshalFlag розбирає JSON з прапорця; порожній прапорець нічого не змінює.
func unmarshalFlag(name, value string, out *any) error {
	if value == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return fmt.Errorf("--%s: невалідний JSON: %w", name, err)
	}
	return nil
}
