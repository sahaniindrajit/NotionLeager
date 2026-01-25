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

func (c *Client) GetExpensesByDateRange(
	start time.Time,
	end time.Time,
) ([]ExpenseRow, error) {

	var all []ExpenseRow
	var cursor *string

	for {
		reqBody := queryRequest{
			PageSize: 100,
		}

		reqBody.Filter.And = []interface{}{
			map[string]interface{}{
				"property": "Date",
				"date": map[string]string{
					"on_or_after": start.Format("2006-01-02"),
				},
			},
			map[string]interface{}{
				"property": "Date",
				"date": map[string]string{
					"on_or_before": end.Format("2006-01-02"),
				},
			},
		}

		reqBody.Sorts = []map[string]string{
			{
				"property":  "Date",
				"direction": "ascending",
			},
		}

		reqBody.StartCursor = cursor

		body, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(
			"POST",
			fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.dbID),
			bytes.NewBuffer(body),
		)

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Notion-Version", "2022-06-28")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			var errBody map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&errBody)
			return nil, fmt.Errorf("notion query error (%d): %v", resp.StatusCode, errBody)
		}

		var res queryResponse
		json.NewDecoder(resp.Body).Decode(&res)

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
	props := raw["properties"].(map[string]interface{})

	// Date
	dateStr := props["Date"].(map[string]interface{})["date"].(map[string]interface{})["start"].(string)
	date, _ := time.Parse("2006-01-02", dateStr)

	// Amount
	amount := props["Amount"].(map[string]interface{})["number"].(float64)

	// Category relation → ID
	categoryID := ""
	if rel, ok := props["Category"].(map[string]interface{})["relation"].([]interface{}); ok && len(rel) > 0 {
		categoryID = rel[0].(map[string]interface{})["id"].(string)
	}

	categoryName := ResolveCategoryName(categoryID)

	return ExpenseRow{
		Date:       date,
		Amount:     amount,
		CategoryID: categoryID,
		Category:   categoryName,
	}, nil
}

func (c *Client) GetLastExpense() (*map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"page_size": 1,
		"sorts": []map[string]string{
			{
				"property":  "Date",
				"direction": "descending",
			},
		},
	}

	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", c.dbID),
		bytes.NewBuffer(body),
	)

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Results []map[string]interface{} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	if len(res.Results) == 0 {
		return nil, nil
	}

	return &res.Results[0], nil
}

func ParseLastExpense(raw map[string]interface{}) LastExpense {
	props := raw["properties"].(map[string]interface{})

	name := props["Name"].(map[string]interface{})["title"].([]interface{})[0].(map[string]interface{})["plain_text"].(string)

	amount := props["Amount"].(map[string]interface{})["number"].(float64)

	date := props["Date"].(map[string]interface{})["date"].(map[string]interface{})["start"].(string)

	categoryID := ""
	rel := props["Category"].(map[string]interface{})["relation"].([]interface{})
	if len(rel) > 0 {
		categoryID = rel[0].(map[string]interface{})["id"].(string)
	}
	category := ResolveCategoryName(categoryID)

	desc := ""
	if rt, ok := props["Description"]; ok {
		arr := rt.(map[string]interface{})["rich_text"].([]interface{})
		if len(arr) > 0 {
			desc = arr[0].(map[string]interface{})["plain_text"].(string)
		}
	}

	return LastExpense{
		PageID:      raw["id"].(string),
		Name:        name,
		Amount:      amount,
		Category:    category,
		Description: desc,
		Date:        date,
	}
}

func (c *Client) DeletePage(pageID string) error {

	body := bytes.NewBufferString(`{"archived": true}`)

	req, _ := http.NewRequest(
		"PATCH",
		"https://api.notion.com/v1/pages/"+pageID,
		body,
	)

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to delete page, status=%d", resp.StatusCode)
	}

	return nil
}
