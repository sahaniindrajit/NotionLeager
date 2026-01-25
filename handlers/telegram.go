package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"notionLeager/config"
	"notionLeager/expense"
	"notionLeager/notion"
	"notionLeager/telegram"
	"notionLeager/utils"
	"strconv"
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

var deduper = expense.NewDeduper(30 * time.Second)
var CategoryResolver *notion.CategoryResolver
var notionClient *notion.Client

func init() {
	cats, fallback := notion.SeedCategories()
	CategoryResolver = notion.NewCategoryResolver(cats, fallback)
	notion.InitCategoryMap()
}

func TelegramWebhook(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		var update TelegramUpdate

		if notionClient == nil {
			notionClient = notion.NewClient(
				cfg.NotionAPIKey,
				cfg.NotionExpenseDB,
			)
		}

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
		key := fmt.Sprintf("%d:%s", update.Message.Chat.ID, text)
		if deduper.Seen(key) {
			return
		}
		log.Println("Owner message", text)

		if text == "/start" {
			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				"👋 Hi!\n\nSend expenses like:\nLunch, 450, Food\nLunch, 450, Food, Office lunch\n\nCurrency: INR",
			)
			return
		}

		if text == "/week" {
			start, end := utils.ThisWeekRange(time.Now())

			rows, err := notionClient.GetExpensesByDateRange(start, end)

			if err != nil {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"❌ Failed to fetch weekly expenses",
				)
				return
			}
			if len(rows) == 0 {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"📊 This Week\n\nNo expenses recorded.",
				)
				return
			}

			totals := expense.AggregateByDay(rows)

			var total float64
			msg := "📊 This Week\n\n"

			for _, d := range totals {
				total += d.Amount
				msg += d.Date.Format("Mon") + "  •  ₹" + fmt.Sprintf("%.2f", d.Amount) + "\n"
			}

			msg = "📊 This Week — ₹" + fmt.Sprintf("%.2f", total) + "\n\n" + msg[11:]

			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				msg,
			)

			return
		}

		if text == "/month" {
			start, end := utils.ThisMonthRange(time.Now())

			rows, err := notionClient.GetExpensesByDateRange(start, end)
			if err != nil {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"❌ Failed to fetch monthly expenses",
				)
				return
			}

			if len(rows) == 0 {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"📊 This Month\n\nNo expenses recorded.",
				)
				return
			}

			totals := expense.AggregateByCategory(rows)

			var total float64
			for _, c := range totals {
				total += c.Amount
			}

			monthLabel := time.Now().Format("January 2006")
			msg := fmt.Sprintf("📊 %s — ₹%.2f\n\n", monthLabel, total)

			for _, c := range totals {
				msg += fmt.Sprintf("%-18s ₹%.2f\n", c.Category, c.Amount)
			}

			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				msg,
			)

			return
		}

		if text == "/summary" {
			start, end := utils.ThisMonthRange(time.Now())

			rows, err := notionClient.GetExpensesByDateRange(start, end)
			if err != nil {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"❌ Failed to fetch summary",
				)
				return
			}

			if len(rows) == 0 {
				telegram.SendMessage(
					cfg.TelegramBotToken,
					update.Message.Chat.ID,
					"📈 Summary\n\nNo expenses recorded this month.",
				)
				return
			}

			s := expense.BuildSummary(rows)
			monthLabel := time.Now().Format("January 2006")

			msg := fmt.Sprintf(
				"📈 Summary — %s\n\n"+
					"• Total spent: ₹%.2f\n"+
					"• Daily average: ₹%.2f\n"+
					"• Highest day: ₹%.2f\n"+
					"• Lowest day: ₹%.2f\n"+
					"• Top category: %s",
				monthLabel,
				s.Total,
				s.DailyAvg,
				s.HighestDay,
				s.LowestDay,
				s.TopCategory,
			)

			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				msg,
			)

			return
		}

		exp, err := expense.Parse(text)
		if err != nil {
			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				"❌ Invalid format\n\nUse:\nName, Amount, Category, Description (optional)",
			)
			return
		}
		category := CategoryResolver.Resolve(exp.CategoryRaw)

		err = notionClient.CreateExpense(
			exp.Name,
			exp.Amount,
			category.ID,
			exp.Description,
		)

		if err != nil {
			// fmt.Printf("Error in saving data in notion ", err)
			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				"❌ Failed to save expense",
			)
			return
		}

		msg := fmt.Sprintf(
			"✅ Saved\n\nName: %s\nAmount: ₹%.2f\nCategory: %s",
			exp.Name,
			exp.Amount,
			category.Name,
		)

		if exp.Description != "" {
			msg += fmt.Sprintf("\nDescription: %s", exp.Description)
		}

		telegram.SendMessage(
			cfg.TelegramBotToken,
			update.Message.Chat.ID,
			msg,
		)

		log.Printf("Parsed expense: %+v\n", exp)
		log.Printf(
			"Resolved category: input=%q → %s (%s)\n",
			exp.CategoryRaw,
			category.Name,
			category.ID,
		)

		w.WriteHeader(http.StatusOK)
	}
}
