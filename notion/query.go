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
