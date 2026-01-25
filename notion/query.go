package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExpenseRow struct {
	Date       time.Time
	Amount     float64
	CategoryID string
	Category   string
}

type queryRequest struct {
	Filter struct {
		And []interface{} `json:"and"`
	} `json:"filter"`
	Sorts       []map[string]string `json:"sorts"`
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
	// Pre-allocate with reasonable capacity
	all := make([]ExpenseRow, 0, 100)
	var cursor *string

	for {
		reqBody := queryRequest{PageSize: 100}
		reqBody.Filter.And = []interface{}{
			map[string]interface{}{
				"property": "Date",
				"date":     map[string]string{"on_or_after": start.Format("2006-01-02")},
			},
			map[string]interface{}{
				"property": "Date",
				"date":     map[string]string{"on_or_before": end.Format("2006-01-02")},
			},
		}
		reqBody.Sorts = []map[string]string{{"property": "Date", "direction": "ascending"}}
		reqBody.StartCursor = cursor

		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequest("POST", fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.dbID), bytes.NewBuffer(body))
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

		if !res.HasMore {
			break
		}
		cursor = res.NextCursor
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

	// Category relation
	categoryID := ""
	if catProp, ok := props["Category"].(map[string]interface{}); ok {
		if rel, ok := catProp["relation"].([]interface{}); ok && len(rel) > 0 {
			if first, ok := rel[0].(map[string]interface{}); ok {
				if id, ok := first["id"].(string); ok {
					categoryID = id
				}
			}
		}
	}

	return ExpenseRow{
		Date:       date,
		Amount:     amount,
		CategoryID: categoryID,
		Category:   ResolveCategoryName(categoryID),
	}, nil
}

func (c *Client) GetLastExpense() (*map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"page_size": 1,
		"sorts":     []map[string]string{{"property": "Date", "direction": "descending"}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.dbID), bytes.NewBuffer(body))
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

	name := ""
	if nameProp, ok := props["Name"].(map[string]interface{}); ok {
		if titles, ok := nameProp["title"].([]interface{}); ok && len(titles) > 0 {
			if first, ok := titles[0].(map[string]interface{}); ok {
				name, _ = first["plain_text"].(string)
			}
		}
	}

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
	if descProp, ok := props["Description"].(map[string]interface{}); ok {
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

	req, err := http.NewRequest("PATCH", "https://api.notion.com/v1/pages/"+pageID, body)
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

	return nil
}

// Helper functions for safe nested map access
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
