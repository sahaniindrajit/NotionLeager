# NotionLedger

A personal Telegram bot for tracking expenses, with Notion as the database backend.

## Features

- Log expenses via Telegram messages
- Automatic category matching
- Weekly and monthly summaries
- Visual pie charts
- Edit and delete recent expenses
- All data stored in your Notion database

## Setup

### 1. Create a Telegram Bot

1. Open Telegram and search for [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts to create your bot
3. Copy the bot token provided

### 2. Get Your Telegram User ID

1. Search for [@userinfobot](https://t.me/userinfobot) on Telegram
2. Start a chat and it will reply with your user ID
3. This ensures only you can use the bot

### 3. Set Up Notion Integration

1. Go to [Notion Integrations](https://www.notion.so/my-integrations)
2. Click **New integration**
3. Give it a name (e.g., "Expense Tracker")
4. Copy the **Internal Integration Token**

### 4. Create Notion Database

Create a database in Notion with these properties:

| Property | Type |
|----------|------|
| Name | Title |
| Amount | Number |
| Category | Relation (to a Categories database) |
| Date | Date |
| Summary | Text |

Then connect your integration:
1. Open the database page
2. Click the `...` menu in the top right
3. Go to **Connections** > Add your integration

### 5. Get Database ID

1. Open your database in Notion
2. Copy the URL - it looks like: `https://notion.so/workspace/abc123def456...?v=...`
3. The database ID is the 32-character string before the `?`

### 6. Configure Environment

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

### 7. Deploy

#### Using Docker

```bash
docker build -t notionledger .
docker run -p 8080:8080 --env-file .env notionledger
```

#### Without Docker

```bash
go build -o notionledger cmd/main.go
./notionledger
```

### 8. Set Up Webhook

Point your Telegram bot's webhook to your server:

```bash
curl "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook?url=https://your-domain.com/webhook"
```

## Usage

### Log an Expense

Send a message in this format:
```
Name, Amount, Category, Description (optional)
```

Examples:
- `Lunch, 450, Food`
- `Uber, 250, Travel, Office commute`
- `Netflix, 649, Subscription`

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome message |
| `/help` | Show all commands |
| `/today` | Today's expenses |
| `/week` | This week's expenses by day |
| `/month` | This month's expenses by category |
| `/summary` | Monthly statistics |
| `/chart` | Pie chart of expenses |
| `/last` | Show last expense |
| `/lastedit` | Edit last expense |
| `/lastdelete` | Delete last expense |
| `/categories` | List all categories and their keywords |
| `/subs` | List configured monthly subscriptions |
| `/addsubs` | Add all monthly subscriptions for the current month |
| `/addsubs YYYY-MM` | Add all monthly subscriptions for a given month |
| `/withdraw 500 [note]` | Record an ATM withdrawal (Cash category) |
| `/cash Name, Amount, Category, [Description]` | Record something paid in cash |
| `/cashleft` | Cash left from recent withdrawals |
| `/cashmonth` | This month's cash spends by category |

### Monthly Subscriptions

Recurring expenses are defined in `expense/subscriptions.go` (`MonthlySubscriptions`).
`/addsubs` adds each one dated on its charge day for the month; if the month is shorter
than the charge day (e.g. the 30th in February) the last day of the month is used.
Running it twice for the same month skips subscriptions that are already recorded.

### Cash Tracking

The main table mirrors your bank account, so an ATM withdrawal is one row there
(category **Cash**). What you buy with that cash goes into a separate **Cash Expenses**
database, each item linked to the withdrawal it came from:

1. `/withdraw 500` → adds "Cash withdrawal ₹500" to the main table.
2. `/cash Tea, 20, Food` → adds Tea to Cash Expenses, linked to your latest withdrawal,
   and replies with how much of it is left.
3. `/cashleft` / `/cashmonth` → balances and category breakdown.

Opening a withdrawal in Notion shows its linked cash items, with **Cash Spent** (rollup)
and **Cash Left** (formula) columns. `/week`, `/month`, `/summary` and `/chart` keep
matching your bank; cash items are reported separately so nothing is counted twice.

Cash Expenses database properties:

| Property | Type |
|----------|------|
| Name | Title |
| Amount | Number |
| Category | Relation (to the Categories database) |
| Date | Date |
| Withdrawal | Relation (to the Expenses database, two-way) |
| Summary | Text |

Set `NOTION_CASH_DB_ID` to its ID and connect your integration to it. Without it,
`/cash`, `/cashleft` and `/cashmonth` reply that cash tracking isn't set up.

### Categories

The bot matches your input to these categories:

- Business
- Shopping
- Entertainment
- Miscellaneous (fallback)
- Cash (ATM withdrawals)
- Donation
- Travel
- Food & Drink
- Education
- Home & Utility
- Health & Supplements
- Subscription
- Insurance
- Family
- EMI

Category matching is flexible - "food" matches "Food & Drink", "travel" matches "Travel", etc.

## License

Personal project - use at your own discretion.
