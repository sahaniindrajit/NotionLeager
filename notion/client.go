package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Shared HTTP client with connection pooling and timeouts
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

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

func (c *Client) CreateExpense(name string, amount float64, categoryID string, description string) error {
	reqBody := createPageRequest{}
	reqBody.Parent.DatabaseID = c.dbID

	props := map[string]interface{}{
		"Name": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": name}},
			},
		},
		"Amount": map[string]interface{}{
			"number": amount,
		},
		"Category": map[string]interface{}{
			"relation": []map[string]string{{"id": categoryID}},
		},
		"Date": map[string]interface{}{
			"date": map[string]string{"start": time.Now().Format("2006-01-02")},
		},
	}

	if description != "" {
		props["Summary"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": description}},
			},
		}
	}

	reqBody.Properties = props

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notion API error: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) UpdatePage(pageID string, properties map[string]interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"properties": properties})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PATCH", "https://api.notion.com/v1/pages/"+pageID, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to update page: status %d", resp.StatusCode)
	}

	return nil
}
