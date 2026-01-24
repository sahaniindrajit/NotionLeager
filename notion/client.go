package notion

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type Client struct {
	apiKey string
	dbID   string
}

func NewClient(apiKey, dbID string) *Client {
	return &Client{
		apiKey: apiKey,
		dbID:   dbID,
	}
}

type createPageRequest struct {
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]interface{} `json:"properties"`
}

func (c *Client) CreateExpense(
	name string,
	amount float64,
	categoryID string,
	description string,
) error {

	reqBody := createPageRequest{}
	reqBody.Parent.DatabaseID = c.dbID

	props := map[string]interface{}{
		"Name": map[string]interface{}{
			"title": []map[string]interface{}{
				{
					"text": map[string]string{
						"content": name,
					},
				},
			},
		},
		"Amount": map[string]interface{}{
			"number": amount,
		},
		"Category": map[string]interface{}{
			"relation": []map[string]string{
				{
					"id": categoryID,
				},
			},
		},
		"Date": map[string]interface{}{
			"date": map[string]string{
				"start": time.Now().Format("2006-01-02"),
			},
		},
	}

	if description != "" {
		props["Summary"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"text": map[string]string{
						"content": description,
					},
				},
			},
		}
	}

	reqBody.Properties = props

	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(
		"POST",
		"https://api.notion.com/v1/pages",
		bytes.NewBuffer(body),
	)

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return err
	}

	return nil
}
