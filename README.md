# elastic-apm-mcp-server

MCP-сервер для перегляду даних Elastic APM через Kibana API: список сервісів, продуктивність транзакцій, помилки та runtime-метрики.

## Вимоги

- Node.js 18 або новіший
- Kibana 8.x (використовуються внутрішні ендпоінти Kibana; поведінка може відрізнятися залежно від версії)

## Установка

Є два способи: **глобально** (рекомендовано для постійного використання) або **через `npx`** (без установки, завжди остання версія з npm).

### Глобальна установка

```bash
npm install -g elastic-apm-mcp-server
```

Перевірте, що бінарник у `PATH`:

```bash
which elastic-apm-mcp-server
```

Якщо `which` нічого не знаходить, додайте глобальний каталог npm до `PATH` (шлях залежить від ОС):

```bash
# macOS / Linux — типовий шлях
export PATH="$(npm config get prefix)/bin:$PATH"
```

Установка з локального клону репозиторію:

```bash
git clone https://github.com/h0rn3t/elastic-apm-mcp-server.git
cd elastic-apm-mcp-server
npm install
npm run build
npm link
```

### Запуск через npx (без установки)

```bash
npx -y elastic-apm-mcp-server
```

## Налаштування MCP

Після установки додайте сервер у конфіг MCP-клієнта. У прикладах нижче замініть URL і облікові дані Kibana на свої.

### Cursor

Файл `~/.cursor/mcp.json` (глобально) або `.cursor/mcp.json` у корені проєкту:

**Глобальна установка:**

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

**Через npx:**

```json
{
  "mcpServers": {
    "elastic-apm": {
      "command": "npx",
      "args": ["-y", "elastic-apm-mcp-server"],
      "env": {
        "KIBANA_URL": "http://your-kibana-host:5601",
        "KIBANA_USERNAME": "your-username",
        "KIBANA_PASSWORD": "your-password"
      }
    }
  }
}
```

Після змін перезапустіть Cursor або перезавантажте вікно MCP (Command Palette → «MCP: List Servers» / перезапуск).

### Claude Code

**Глобальна установка** — через CLI:

```bash
claude mcp add elastic-apm \
  -e KIBANA_URL=http://your-kibana-host:5601 \
  -e KIBANA_USERNAME=your-username \
  -e KIBANA_PASSWORD=your-password \
  -- elastic-apm-mcp-server
```

**Через npx:**

```bash
claude mcp add elastic-apm \
  -e KIBANA_URL=http://your-kibana-host:5601 \
  -e KIBANA_USERNAME=your-username \
  -e KIBANA_PASSWORD=your-password \
  -- npx -y elastic-apm-mcp-server
```

Або вручну в `~/.claude/settings.json` (приклад для глобальної установки):

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

### Claude Desktop

На macOS додайте той самий блок `mcpServers` у `~/Library/Application Support/Claude/claude_desktop_config.json` — використовуйте `"command": "elastic-apm-mcp-server"` після глобальної установки або `"command": "npx"` з `"args": ["-y", "elastic-apm-mcp-server"]` без неї.

### Змінні середовища

| Змінна | Обов'язкова | Опис |
|--------|-------------|------|
| `KIBANA_URL` | Так | URL для доступу до Kibana |
| `KIBANA_USERNAME` | Так | Ім'я користувача Kibana |
| `KIBANA_PASSWORD` | Так | Пароль Kibana |

## Доступні інструменти

| Інструмент | Опис |
|------------|------|
| `list_services` | Список сервісів, зареєстрованих в APM |
| `get_service_overview` | Короткий огляд сервісу: продуктивність транзакцій і помилки |
| `get_transactions` | Статистика продуктивності транзакцій для сервісу |
| `get_errors` | Перевірка сервісу на помилки (достатньо `serviceName`; за замовчуванням — останні 24 год) |
| `check_service_errors` | Те саме, що `get_errors` — зручніший вибір для запитів «перевір на помилки» |
| `get_service_metrics` | Runtime-метрики (CPU, пам'ять тощо); набір залежить від типу агента |
| `get_transaction_samples` | Зразки трейсів (traceId) для транзакції; можна відфільтрувати повільні запити |
| `get_trace` | Повний waterfall span за trace ID для пошуку вузьких місць |

## Приклади використання

Після налаштування MCP можна звертатися до Claude природною мовою, наприклад:

- «Покажи список APM-сервісів»
- «Перевір auth-service на помилки» (MCP викличе `check_service_errors` або `get_errors` з `serviceName: auth-service`)
- «Є помилки в payment-service за останню годину?» (`serviceName: payment-service`, `period: 1h`)
- «Перевір продуктивність транзакцій payment-service»
- «Знайди найповільніший запит у payment-service і покажи, де витрачається час»

### Перевірка сервісу на помилки

Достатньо вказати **назву сервісу** як у Elastic APM (наприклад `auth-service`). Часовий діапазон необов'язковий:

| Параметр | Опис |
|----------|------|
| `serviceName` | Обов'язково — ім'я APM-сервісу |
| `period` | `15m`, `1h`, `6h`, `24h`, `7d` (за замовчуванням `24h`) |
| `hours` | Довільний інтервал у годинах (1–168), якщо `period` не підходить |
| `environment` | Фільтр середовища (`production`, `staging`, …) |

Якщо назва сервісу невідома, спочатку запитайте список: «покажи APM-сервіси» → `list_services`.

## Ліцензія

ISC
