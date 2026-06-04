# elastic-apm-mcp-server

Один Go-бінарник для перегляду даних **Elastic APM** через Kibana API у двох режимах:

- **MCP-сервер** через stdio (JSON-RPC) — для Cursor, Claude Desktop, Claude Code та інших MCP-клієнтів;
- **CLI** з підкомандами — для shell або виклику агентом через Bash.

Список сервісів, продуктивність транзакцій, помилки, runtime-метрики та аналіз трейсів (waterfall).

## Інструменти MCP

| Інструмент | Опис |
|------------|------|
| `list_services` | Список сервісів, зареєстрованих в APM |
| `get_service_overview` | Огляд сервісу: транзакції + помилки |
| `get_transactions` | Статистика продуктивності транзакцій |
| `get_errors` | Перевірка сервісу на помилки (достатньо `serviceName`; за замовчуванням — останні 24 год) |
| `check_service_errors` | Те саме, що `get_errors` |
| `get_service_metrics` | Runtime-метрики (CPU, пам'ять тощо) |
| `get_transaction_samples` | Зразки трейсів для транзакції |
| `get_trace` | Повний waterfall span за trace ID |

## Два способи запуску

```bash
elastic-apm-mcp-server              # MCP stdio (за замовчуванням)
elastic-apm-mcp-server mcp          # MCP stdio (явно)
elastic-apm-mcp-server help         # Довідка CLI
elastic-apm-mcp-server <команда>    # CLI-режим
```

### CLI

```bash
elastic-apm-mcp-server list-services --start 2026-06-01T00:00:00Z --end 2026-06-04T00:00:00Z
elastic-apm-mcp-server get-service-overview --service auth-service --start ... --end ...
elastic-apm-mcp-server get-errors --service payment-service --period 1h
elastic-apm-mcp-server get-transactions --service payment-service --start ... --end ...
elastic-apm-mcp-server get-service-metrics --service payment-service --agent java --start ... --end ...
elastic-apm-mcp-server get-transaction-samples --service payment-service --transaction "GET /api" --start ... --end ...
elastic-apm-mcp-server get-trace --trace-id abc123 --start ... --end ...
```

Прапорці мають йти **перед** позиційними аргументами. Коди виходу: `0` — успіх, `1` — помилка операції, `2` — помилка використання.

## Вимоги

- Go 1.26+
- Kibana 8.x (внутрішні ендпоінти APM; поведінка може відрізнятися за версією)

## Установка

### З вихідного коду

```bash
git clone https://github.com/h0rn3t/elastic-apm-mcp-server.git
cd elastic-apm-mcp-server
make install   # збирає та встановлює в /usr/local/bin/
```

### Go install

```bash
go install github.com/h0rn3t/elastic-apm-mcp-server/cmd/elastic-apm-mcp-server@latest
```

## Налаштування MCP

### Cursor

Файл `~/.cursor/mcp.json` (глобально) або `.cursor/mcp.json` у корені проєкту:

```json
{
  "mcpServers": {
    "elastic-apm": {
      "command": "elastic-apm-mcp-server",
      "env": {
        "KIBANA_URL": "http://your-kibana-host:5601",
        "KIBANA_USERNAME": "your-username",
        "KIBANA_PASSWORD": "your-password"
      }
    }
  }
}
```

Після змін перезапустіть Cursor або перезавантажте MCP-сервери.

### Claude Code

```bash
claude mcp add elastic-apm \
  -e KIBANA_URL=http://your-kibana-host:5601 \
  -e KIBANA_USERNAME=your-username \
  -e KIBANA_PASSWORD=your-password \
  -- elastic-apm-mcp-server
```

### Змінні середовища

| Змінна | Обов'язкова | Опис |
|--------|-------------|------|
| `KIBANA_URL` | Так | URL для доступу до Kibana |
| `KIBANA_USERNAME` | Так | Ім'я користувача Kibana |
| `KIBANA_PASSWORD` | Так | Пароль Kibana |

## Приклади (природна мова в MCP-клієнті)

- «Покажи список APM-сервісів»
- «Перевір auth-service на помилки»
- «Є помилки в payment-service за останню годину?» (`period: 1h`)
- «Знайди найповільніший запит у payment-service і покажи, де витрачається час»

### Часовий діапазон для помилок

| Параметр | Опис |
|----------|------|
| `serviceName` | Обов'язково — ім'я APM-сервісу |
| `period` | `15m`, `1h`, `6h`, `24h`, `7d` (за замовчуванням `24h`) |
| `hours` | Довільний інтервал у годинах (1–168) |
| `environment` | Фільтр середовища |

## Ліцензія

ISC
