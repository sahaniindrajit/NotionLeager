package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExpenseRow struct {
	ID           string
	WithdrawalID string // Cash Expenses only: the withdrawal page this was paid from
	Name         string
	Date         time.Time
	Amount       float64
	CategoryID   string
	Category     string
}

type queryRequest struct {
	Filter struct {
		And []interface{} `json:"and"`
	} `json:"filter"`
	Sorts       []map[string]string `json:"sorts,omitempty"`
	StartCursor *string             `json:"start_cursor,omitempty"`
	PageSize    int                 `json:"page_size"`
}

type queryResponse struct {
	Results    []map[string]interface{} `json:"results"`
	HasMore    bool                     `json:"has_more"`
	NextCursor *string                  `json:"next_cursor"`
}

type LastExpense struct {
	PageID      string
	Name        string
	Amount      float64
	Category    string
	Description string
	Date        string
}

func (c *Client) GetExpensesByDateRange(start time.Time, end time.Time) ([]ExpenseRow, error) {
	return c.query(c.dbID, dateRangeFilter(start, end), dateAscending, 0)
}

// GetCashExpensesByDateRange returns Cash Expenses items dated within the range.
func (c *Client) GetCashExpensesByDateRange(start time.Time, end time.Time) ([]ExpenseRow, error) {
	return c.query(c.cashDBID, dateRangeFilter(start, end), dateAscending, 0)
}

// GetWithdrawals returns the most recent cash withdrawals (Cash category), newest first.
func (c *Client) GetWithdrawals(limit int) ([]ExpenseRow, error) {
	filter := []interface{}{relationContains("Category", CashCategoryID)}
	sorts := []map[string]string{
		{"property": "Date", "direction": "descending"},
		{"timestamp": "created_time", "direction": "descending"},
	}
	return c.query(c.dbID, filter, sorts, limit)
}

// GetCashSpentFrom returns the total of Cash Expenses items linked to a withdrawal.
func (c *Client) GetCashSpentFrom(withdrawalID string) (float64, error) {
	rows, err := c.query(c.cashDBID, []interface{}{relationContains("Withdrawal", withdrawalID)}, nil, 0)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, r := range rows {
		total += r.Amount
	}
	return total, nil
}

var dateAscending = []map[string]string{{"property": "Date", "direction": "ascending"}}

func dateRangeFilter(start, end time.Time) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"property": "Date",
			"date":     map[string]string{"on_or_after": start.Format("2006-01-02")},
		},
		map[string]interface{}{
			"property": "Date",
			"date":     map[string]string{"on_or_before": end.Format("2006-01-02")},
		},
	}
}

func relationContains(property, pageID string) map[string]interface{} {
	return map[string]interface{}{
		"property": property,
		"relation": map[string]string{"contains": pageID},
	}
}

// query runs a filtered database query, following pagination. limit 0 means no limit.
func (c *Client) query(dbID string, filters []interface{}, sorts []map[string]string, limit int) ([]ExpenseRow, error) {
	// Pre-allocate with reasonable capacity
	all := make([]ExpenseRow, 0, 100)
	var cursor *string

	for {
		reqBody := queryRequest{PageSize: 100}
		if limit > 0 && limit < 100 {
			reqBody.PageSize = limit
		}
		reqBody.Filter.And = filters
		reqBody.Sorts = sorts
		reqBody.StartCursor = cursor

		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/databases/%s/query", APIBaseURL, dbID), bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Notion-Version", "2022-06-28")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 300 {
			var errBody map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&errBody)
			resp.Body.Close()
			return nil, fmt.Errorf("notion query error (%d): %v", resp.StatusCode, errBody)
		}

		var res queryResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for _, r := range res.Results {
			row, err := parseExpenseRow(r)
			if err == nil {
				all = append(all, row)
			}
		}

		if !res.HasMore || (limit > 0 && len(all) >= limit) {
			break
		}
		cursor = res.NextCursor
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func parseExpenseRow(raw map[string]interface{}) (ExpenseRow, error) {
	props, ok := raw["properties"].(map[string]interface{})
	if !ok {
		return ExpenseRow{}, fmt.Errorf("invalid properties")
	}

	// Date - with safe type assertions
	dateStr := getNestedString(props, "Date", "date", "start")
	date, _ := time.Parse("2006-01-02", dateStr)

	// Amount
	amount := getNestedFloat(props, "Amount", "number")

	categoryID := firstRelationID(props, "Category")
	id, _ := raw["id"].(string)

	return ExpenseRow{
		ID:           id,
		WithdrawalID: firstRelationID(props, "Withdrawal"),
		Name:         getTitle(props, "Name"),
		Date:         date,
		Amount:       amount,
		CategoryID:   categoryID,
		Category:     ResolveCategoryName(categoryID),
	}, nil
}

// GetLastExpense returns the most recently created expense across the main table
// and, when configured, the Cash Expenses table. isCash reports which one it came from.
func (c *Client) GetLastExpense() (raw *map[string]interface{}, isCash bool, err error) {
	if raw, isCash, err = c.newestRemembered(); raw != nil || err != nil {
		return raw, isCash, err
	}

	raw, err = c.lastCreated(c.dbID)
	if err != nil || !c.HasCashDB() {
		return raw, false, err
	}

	cash, err := c.lastCreated(c.cashDBID)
	if err != nil {
		return nil, false, err
	}
	if cash != nil && (raw == nil || createdTime(*cash) > createdTime(*raw)) {
		return cash, true, nil
	}
	return raw, false, nil
}

// createdTime returns a page's created_time. Notion formats it the same way for
// every page, so the strings compare in time order.
func createdTime(page map[string]interface{}) string {
	t, _ := page["created_time"].(string)
	return t
}

// lastCreated returns the newest page in a database by creation time, or nil if it is empty.
func (c *Client) lastCreated(dbID string) (*map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"page_size": 1,
		"sorts":     []map[string]string{{"timestamp": "created_time", "direction": "descending"}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/databases/%s/query", APIBaseURL, dbID), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("notion query failed: %s", resp.Status)
	}

	var res struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(res.Results) == 0 {
		return nil, nil
	}

	return &res.Results[0], nil
}

func ParseLastExpense(raw map[string]interface{}) LastExpense {
	props, _ := raw["properties"].(map[string]interface{})

	name := getTitle(props, "Name")
	amount := getNestedFloat(props, "Amount", "number")
	date := getNestedString(props, "Date", "date", "start")

	categoryID := ""
	if catProp, ok := props["Category"].(map[string]interface{}); ok {
		if rel, ok := catProp["relation"].([]interface{}); ok && len(rel) > 0 {
			if first, ok := rel[0].(map[string]interface{}); ok {
				categoryID, _ = first["id"].(string)
			}
		}
	}

	desc := ""
	if descProp, ok := props["Summary"].(map[string]interface{}); ok {
		if arr, ok := descProp["rich_text"].([]interface{}); ok && len(arr) > 0 {
			if first, ok := arr[0].(map[string]interface{}); ok {
				desc, _ = first["plain_text"].(string)
			}
		}
	}

	pageID, _ := raw["id"].(string)

	return LastExpense{
		PageID:      pageID,
		Name:        name,
		Amount:      amount,
		Category:    ResolveCategoryName(categoryID),
		Description: desc,
		Date:        date,
	}
}

func (c *Client) DeletePage(pageID string) error {
	body := bytes.NewBufferString(`{"archived": true}`)

	req, err := http.NewRequest("PATCH", APIBaseURL+"/pages/"+pageID, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to delete page, status=%d", resp.StatusCode)
	}

	c.forget(pageID)
	return nil
}

// Helper functions for safe nested map access
func firstRelationID(props map[string]interface{}, key string) string {
	if prop, ok := props[key].(map[string]interface{}); ok {
		if rel, ok := prop["relation"].([]interface{}); ok && len(rel) > 0 {
			if first, ok := rel[0].(map[string]interface{}); ok {
				id, _ := first["id"].(string)
				return id
			}
		}
	}
	return ""
}

func getTitle(props map[string]interface{}, key string) string {
	if prop, ok := props[key].(map[string]interface{}); ok {
		if titles, ok := prop["title"].([]interface{}); ok && len(titles) > 0 {
			if first, ok := titles[0].(map[string]interface{}); ok {
				title, _ := first["plain_text"].(string)
				return title
			}
		}
	}
	return ""
}

func getNestedString(m map[string]interface{}, keys ...string) string {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := current[key].(string); ok {
				return val
			}
			return ""
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

func getNestedFloat(m map[string]interface{}, keys ...string) float64 {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := current[key].(float64); ok {
				return val
			}
			return 0
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return 0
		}
	}
	return 0
}
