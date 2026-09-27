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

// APIBaseURL is the Notion API root. It is a variable so tests can point it at a fake server.
var APIBaseURL = "https://api.notion.com/v1"

type Client struct {
	apiKey   string
	dbID     string
	cashDBID string
}

// NewClient creates a Notion client. cashDBID is the Cash Expenses database;
// it may be empty, in which case cash tracking is disabled.
func NewClient(apiKey, dbID, cashDBID string) *Client {
	return &Client{
		apiKey:   apiKey,
		dbID:     dbID,
		cashDBID: cashDBID,
	}
}

// HasCashDB reports whether a Cash Expenses database is configured.
func (c *Client) HasCashDB() bool {
	return c.cashDBID != ""
}

type createPageRequest struct {
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]interface{} `json:"properties"`
}

func (c *Client) CreateExpense(name string, amount float64, categoryID string, description string) error {
	return c.CreateExpenseOn(time.Now(), name, amount, categoryID, description)
}

// CreateExpenseOn creates an expense dated on the given day.
func (c *Client) CreateExpenseOn(date time.Time, name string, amount float64, categoryID string, description string) error {
	return c.createPage(c.dbID, expenseProps(date, name, amount, categoryID, description))
}

// CreateCashExpense adds an item paid in cash, linked to the withdrawal page it came from.
func (c *Client) CreateCashExpense(date time.Time, name string, amount float64, categoryID, description, withdrawalID string) error {
	props := expenseProps(date, name, amount, categoryID, description)
	props["Withdrawal"] = map[string]interface{}{
		"relation": []map[string]string{{"id": withdrawalID}},
	}
	return c.createPage(c.cashDBID, props)
}

func expenseProps(date time.Time, name string, amount float64, categoryID string, description string) map[string]interface{} {
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
			"date": map[string]string{"start": date.Format("2006-01-02")},
		},
	}

	if description != "" {
		props["Summary"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": description}},
			},
		}
	}

	return props
}

func (c *Client) createPage(dbID string, props map[string]interface{}) error {
	reqBody := createPageRequest{}
	reqBody.Parent.DatabaseID = dbID
	reqBody.Properties = props

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", APIBaseURL+"/pages", bytes.NewBuffer(body))
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

	req, err := http.NewRequest("PATCH", APIBaseURL+"/pages/"+pageID, bytes.NewBuffer(body))
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
