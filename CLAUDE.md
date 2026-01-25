# NotionLedger

Personal Telegram bot for expense tracking with Notion as the database backend.

## Project Structure

```
cmd/main.go           - Entry point, HTTP server setup
config/config.go      - Environment configuration loader
handlers/
  telegram.go         - Webhook handler, all bot commands
  state.go            - Edit session state management
notion/
  client.go           - Notion API client (create/update/delete)
  query.go            - Query operations (fetch expenses)
  categories.go       - Category resolution logic
  category_map.go     - Category ID-to-Name mapping
  seed.go             - Hardcoded category definitions
expense/
  parser.go           - Parse expense text format
  aggregate.go        - Aggregate by day/category
  dedupe.go           - Webhook deduplication
telegram/client.go    - Telegram API client
charts/pie.go         - Pie chart generation
utils/date.go         - Date range utilities
```

## Key Patterns

- **Expense format**: `Name, Amount, Category, [Description]`
- **Currency**: INR (Indian Rupees)
- **Category matching**: Exact match > Substring match > Fallback to "Miscellaneous"
- **Deduplication**: 30-second window to prevent duplicate webhook processing

## Commands

- `/start`, `/help` - Show usage
- `/week` - Weekly day-wise summary
- `/month` - Monthly category breakdown
- `/summary` - Monthly statistics
- `/chart` - Pie chart of expenses
- `/last` - Show last expense
- `/lastedit` - Edit last expense (interactive)
- `/last delete` - Delete last expense

## Development

```bash
go run cmd/main.go
```

Webhook endpoint: `POST /webhook`

## Environment Variables

See `.env.example` for all required variables.
