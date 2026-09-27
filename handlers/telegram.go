package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"notionLeager/charts"
	"notionLeager/config"
	"notionLeager/expense"
	"notionLeager/notion"
	"notionLeager/telegram"
	"notionLeager/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int    `json:"message_id"`
		Text      string `json:"text"`
		From      struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

var (
	deduper          = expense.NewDeduper(30 * time.Second)
	CategoryResolver *notion.CategoryResolver
	notionClient     *notion.Client
	clientOnce       sync.Once
)

func init() {
	cats, fallback := notion.SeedCategories()
	CategoryResolver = notion.NewCategoryResolver(cats, fallback)
	notion.InitCategoryMap()
}

// bot wraps config and chat ID for cleaner message sending
type bot struct {
	token  string
	chatID int64
}

func (b bot) send(msg string) {
	telegram.SendMessage(b.token, b.chatID, msg)
}

func (b bot) sendPhoto(path, caption string) {
	telegram.SendPhoto(b.token, b.chatID, path, caption)
}

func TelegramWebhook(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var update TelegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Println("Invalid telegram update:", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		if update.Message == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		ownerID, _ := strconv.ParseInt(cfg.TelegramOwnerId, 10, 64)
		if update.Message.From.ID != ownerID {
			w.WriteHeader(http.StatusOK)
			return
		}

		text := update.Message.Text
		// Use message ID for deduplication (prevents webhook duplicates, not repeated commands)
		key := fmt.Sprintf("%d:%d", update.Message.Chat.ID, update.Message.MessageID)
		if deduper.Seen(key) {
			return
		}

		utils.Debugln("Owner message", text)

		// Thread-safe client initialization (only once)
		clientOnce.Do(func() {
			notionClient = notion.NewClient(cfg.NotionAPIKey, cfg.NotionExpenseDB)
		})

		b := bot{token: cfg.TelegramBotToken, chatID: update.Message.Chat.ID}

		// Handle edit session flow
		if handleEditSession(b, text) {
			return
		}

		// Route commands: first word is the command, the rest is its argument
		cmd, arg, _ := strings.Cut(strings.TrimSpace(text), " ")
		cmd, _, _ = strings.Cut(cmd, "@") // "/today@MyBot" → "/today"
		arg = strings.TrimSpace(arg)

		switch cmd {
		case "/start":
			handleStart(b)
		case "/help":
			handleHelp(b)
		case "/week":
			handleWeek(b)
		case "/month":
			handleMonth(b)
		case "/summary":
			handleSummary(b)
		case "/chart":
			handleChart(b)
		case "/last":
			handleLast(b)
		case "/lastedit":
			handleLastEdit(b)
		case "/lastdelete":
			handleLastDelete(b)
		case "/today":
			handleToday(b)
		case "/categories":
			handleCategories(b)
		case "/subs":
			handleSubs(b)
		case "/addsubs":
			handleAddSubs(b, arg)
		default:
			handleExpense(b, text)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleStart(b bot) {
	b.send("👋 Hi!\n\nSend expenses like:\nLunch, 450, Food\nLunch, 450, Food, Office lunch\n\nCurrency: INR")
}

func handleHelp(b bot) {
	b.send(`🤖 NotionLedger Bot — Help

Add expense:
• Lunch, 450, Food
• Lunch, 450, Food, Office lunch

Commands:
• /today    → Today's expenses
• /week     → This week's expenses (day-wise)
• /month    → This month's expenses (category-wise)
• /summary  → Monthly spending summary
• /chart    → Monthly category pie chart
• /last     → View last expense
• /lastedit → Edit last expense
• /lastdelete → Delete last expense
• /categories → List all categories
• /subs     → List monthly subscriptions
• /addsubs  → Add this month's subscriptions
• /addsubs 2026-10 → Add subscriptions for a given month
• /help     → Show this help
 
Notes:
• Amount is in INR (₹)
• Date defaults to today
• Only your messages are processed`)
}

func handleToday(b bot) {
	now := time.Now()
	rows, err := notionClient.GetExpensesByDateRange(now, now)
	if err != nil {
		b.send("❌ Failed to fetch today's expenses")
		return
	}

	title := "🗓 Today — " + now.Format("Mon, 02 Jan")
	if len(rows) == 0 {
		b.send(title + "\n\nNo expenses recorded.")
		return
	}

	var total float64
	var sb strings.Builder
	for _, r := range rows {
		total += r.Amount
		fmt.Fprintf(&sb, "• %s — ₹%.2f (%s)\n", r.Name, r.Amount, r.Category)
	}

	b.send(fmt.Sprintf("%s — ₹%.2f\n\n%s", title, total, sb.String()))
}

func handleCategories(b bot) {
	var sb strings.Builder
	sb.WriteString("🏷 Categories\n")
	for _, c := range CategoryResolver.Categories() {
		fmt.Fprintf(&sb, "\n• %s", c.Name)
		if kws := CategoryResolver.Keywords(c.Name); len(kws) > 0 {
			fmt.Fprintf(&sb, "\n   %s", strings.Join(kws, ", "))
		}
	}
	sb.WriteString("\n\nUnmatched categories go to Miscellaneous.")
	b.send(sb.String())
}

func handleSubs(b bot) {
	var total float64
	var sb strings.Builder
	for _, s := range expense.MonthlySubscriptions {
		total += s.Amount
		fmt.Fprintf(&sb, "• %s — ₹%.2f on day %d\n", s.Name, s.Amount, s.Day)
	}
	b.send(fmt.Sprintf("🔁 Monthly Subscriptions — ₹%.2f\n\n%s\nUse /addsubs to add them for this month.", total, sb.String()))
}

// handleAddSubs adds every monthly subscription for the given month ("YYYY-MM",
// or the current month when empty), skipping any already recorded on its date.
func handleAddSubs(b bot, arg string) {
	now := time.Now()
	year, month := now.Year(), now.Month()
	if arg != "" {
		t, err := time.ParseInLocation("2006-01", arg, now.Location())
		if err != nil {
			b.send("❌ Invalid month\n\nUse /addsubs or /addsubs YYYY-MM")
			return
		}
		year, month = t.Year(), t.Month()
	}

	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, -1)
	existing, err := notionClient.GetExpensesByDateRange(monthStart, monthEnd)
	if err != nil {
		b.send("❌ Failed to check existing expenses, nothing added")
		return
	}

	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[subKey(e.Name, e.Date)] = true
	}

	var added, skipped, failed strings.Builder
	var total float64
	for _, s := range expense.MonthlySubscriptions {
		date := s.DateIn(year, month, now.Location())
		line := fmt.Sprintf("• %s — ₹%.2f on %s\n", s.Name, s.Amount, date.Format("02 Jan"))

		if seen[subKey(s.Name, date)] {
			skipped.WriteString(line)
			continue
		}

		category := CategoryResolver.Resolve(s.Category)
		if err := notionClient.CreateExpenseOn(date, s.Name, s.Amount, category.ID, ""); err != nil {
			log.Printf("Failed to add subscription %s: %v", s.Name, err)
			failed.WriteString(line)
			continue
		}
		total += s.Amount
		added.WriteString(line)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🔁 Subscriptions — %s\n", monthStart.Format("January 2006"))
	if added.Len() > 0 {
		fmt.Fprintf(&sb, "\n✅ Added (₹%.2f)\n%s", total, added.String())
	}
	if skipped.Len() > 0 {
		fmt.Fprintf(&sb, "\n⏭ Already added\n%s", skipped.String())
	}
	if failed.Len() > 0 {
		fmt.Fprintf(&sb, "\n❌ Failed\n%s", failed.String())
	}
	b.send(sb.String())
}

func subKey(name string, date time.Time) string {
	return strings.ToLower(strings.TrimSpace(name)) + "|" + date.Format("2006-01-02")
}

func handleWeek(b bot) {
	start, end := utils.ThisWeekRange(time.Now())
	rows, err := notionClient.GetExpensesByDateRange(start, end)
	if err != nil {
		b.send("❌ Failed to fetch weekly expenses")
		return
	}

	if len(rows) == 0 {
		b.send("📊 This Week\n\nNo expenses recorded.")
		return
	}

	totals := expense.AggregateByDay(rows)
	var total float64
	var sb strings.Builder

	for _, d := range totals {
		total += d.Amount
		fmt.Fprintf(&sb, "%s  •  ₹%.2f\n", d.Date.Format("Mon"), d.Amount)
	}

	b.send(fmt.Sprintf("📊 This Week — ₹%.2f\n\n%s", total, sb.String()))
}

func handleMonth(b bot) {
	start, end := utils.ThisMonthRange(time.Now())
	rows, err := notionClient.GetExpensesByDateRange(start, end)
	if err != nil {
		b.send("❌ Failed to fetch monthly expenses")
		return
	}

	if len(rows) == 0 {
		b.send("📊 This Month\n\nNo expenses recorded.")
		return
	}

	totals := expense.AggregateByCategory(rows)
	var total float64
	for _, c := range totals {
		total += c.Amount
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "📊 %s — ₹%.2f\n\n", time.Now().Format("January 2006"), total)
	for _, c := range totals {
		fmt.Fprintf(&sb, "%-18s ₹%.2f\n", c.Category, c.Amount)
	}

	b.send(sb.String())
}

func handleSummary(b bot) {
	start, end := utils.ThisMonthRange(time.Now())
	rows, err := notionClient.GetExpensesByDateRange(start, end)
	if err != nil {
		b.send("❌ Failed to fetch summary")
		return
	}

	if len(rows) == 0 {
		b.send("📈 Summary\n\nNo expenses recorded this month.")
		return
	}

	s := expense.BuildSummary(rows)
	b.send(fmt.Sprintf(`📈 Summary — %s

• Total spent: ₹%.2f
• Daily average: ₹%.2f
• Highest day: ₹%.2f
• Lowest day: ₹%.2f
• Top category: %s`,
		time.Now().Format("January 2006"),
		s.Total, s.DailyAvg, s.HighestDay, s.LowestDay, s.TopCategory))
}

func handleChart(b bot) {
	start, end := utils.ThisMonthRange(time.Now())
	rows, err := notionClient.GetExpensesByDateRange(start, end)
	if err != nil || len(rows) == 0 {
		b.send("❌ No data available for chart")
		return
	}

	totals := expense.AggregateByCategory(rows)
	chartPath := "/tmp/monthly_chart.png"

	if err := charts.GenerateCategoryPie(totals, chartPath); err != nil {
		b.send("❌ Failed to generate chart")
		return
	}

	b.sendPhoto(chartPath, fmt.Sprintf("📊 %s — Category-wise expenses", time.Now().Format("January 2006")))
}

func handleLast(b bot) {
	raw, err := notionClient.GetLastExpense()
	if err != nil || raw == nil {
		b.send("No expenses found.")
		return
	}

	e := notion.ParseLastExpense(*raw)
	var sb strings.Builder
	fmt.Fprintf(&sb, "🧾 Last Expense\n\nName: %s\nAmount: ₹%.2f\nCategory: %s\nDate: %s",
		e.Name, e.Amount, e.Category, e.Date)

	if e.Description != "" {
		sb.WriteString("\nDescription: ")
		sb.WriteString(e.Description)
	}

	b.send(sb.String())
}

func handleLastEdit(b bot) {
	raw, err := notionClient.GetLastExpense()
	if err != nil || raw == nil {
		b.send("No expense found to edit.")
		return
	}

	e := notion.ParseLastExpense(*raw)
	setEditSession(b.chatID, &EditState{PageID: e.PageID})

	b.send("✏️ What do you want to edit?\n\nName | Amount | Category | Description")
}

func handleLastDelete(b bot) {
	raw, err := notionClient.GetLastExpense()
	if err != nil || raw == nil {
		b.send("No expense found to delete.")
		return
	}

	e := notion.ParseLastExpense(*raw)
	if err := notionClient.DeletePage(e.PageID); err != nil {
		b.send("❌ Failed to delete expense")
		return
	}

	b.send(fmt.Sprintf("🗑 Deleted last expense:\n%s — ₹%.2f", e.Name, e.Amount))
}

func handleEditSession(b bot, text string) bool {
	state, ok := getEditSession(b.chatID)
	if !ok {
		return false
	}

	// If user sends a command, cancel edit session and let command through
	if strings.HasPrefix(text, "/") {
		deleteEditSession(b.chatID)
		return false
	}

	// Waiting for field selection
	if state.Field == "" {
		switch text {
		case "Name", "Amount", "Category", "Description":
			state.Field = text
			setEditSession(b.chatID, state) // Update with field set
			b.send("Enter new " + text + ":")
		default:
			b.send("Please choose: Name | Amount | Category | Description")
		}
		return true
	}

	// Process field update
	props := buildUpdateProps(state.Field, text, b)
	if props == nil {
		return true
	}

	deleteEditSession(b.chatID)

	if err := notionClient.UpdatePage(state.PageID, props); err != nil {
		b.send("❌ Failed to update expense")
		return true
	}

	b.send("✅ Expense updated successfully")
	return true
}

func buildUpdateProps(field, value string, b bot) map[string]interface{} {
	props := map[string]interface{}{}

	switch field {
	case "Name":
		props["Name"] = map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": value}},
			},
		}
	case "Amount":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			b.send("Invalid amount.")
			return nil
		}
		props["Amount"] = map[string]interface{}{"number": val}
	case "Category":
		cat := CategoryResolver.Resolve(value)
		props["Category"] = map[string]interface{}{
			"relation": []map[string]string{{"id": cat.ID}},
		}
	case "Description":
		props["Description"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": value}},
			},
		}
	}

	return props
}

func handleExpense(b bot, text string) {
	exp, err := expense.Parse(text)
	if err != nil {
		b.send("❌ Invalid format\n\nUse:\nName, Amount, Category, Description (optional)")
		return
	}

	category := CategoryResolver.Resolve(exp.CategoryRaw)

	if err := notionClient.CreateExpense(exp.Name, exp.Amount, category.ID, exp.Description); err != nil {
		b.send("❌ Failed to save expense")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "✅ Saved\n\nName: %s\nAmount: ₹%.2f\nCategory: %s", exp.Name, exp.Amount, category.Name)
	if exp.Description != "" {
		sb.WriteString("\nDescription: ")
		sb.WriteString(exp.Description)
	}

	b.send(sb.String())

	utils.Debugf("Parsed expense: %+v\n", exp)
	utils.Debugf("Resolved category: input=%q → %s (%s)\n", exp.CategoryRaw, category.Name, category.ID)
}
