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
  subscriptions.go    - Monthly recurring subscriptions (/addsubs)
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
- `/today` - Today's expenses
- `/week` - Weekly day-wise summary
- `/month` - Monthly category breakdown
- `/summary` - Monthly statistics
- `/chart` - Pie chart of expenses
- `/last` - Show the most recently added expense (main or Cash Expenses table)
- `/lastedit` - Edit the most recently added expense (interactive)
- `/lastdelete` - Delete the most recently added expense
- `/categories` - List categories and their keywords
- `/subs` - List monthly subscriptions
- `/addsubs [YYYY-MM]` - Add monthly subscriptions for the current (or given) month; skips ones already added
- `/withdraw 500 [note]` - Record an ATM withdrawal in the main table (Cash category)
- `/cash Name, Amount, Category, [Description]` - Record a cash spend in the Cash Expenses DB, linked to the latest withdrawal
- `/cashleft` - Remaining cash per recent withdrawal
- `/cashmonth` - This month's cash spends by category

## Cash Tracking

- Main DB = bank transactions only; a withdrawal is one row with category Cash (`notion.CashCategoryID`).
- Cash Expenses DB (`NOTION_CASH_DB_ID`, optional) holds items paid in cash, each with a
  `Withdrawal` relation to the withdrawal page. Cash items are never added to the main DB,
  so bank reports don't double count.

## Development

```bash
go run cmd/main.go
go test ./...
```

Tests in `handlers/telegram_test.go` drive the webhook against fake Notion/Telegram servers
(`notion.APIBaseURL` / `telegram.APIBaseURL` are overridden).

Webhook endpoint: `POST /webhook`

## Environment Variables

See `.env.example` for all required variables.
