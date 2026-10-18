package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
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

	// recent holds pages this bot created, newest last, so /last, /lastedit and
	// /lastdelete know the true order. Notion's created_time only has minute
	// precision, so it can't tell apart two expenses added in the same minute.
	// The list is lost on restart; GetLastExpense then falls back to created_time.
	recentMu sync.Mutex
	recent   []recentPage
}

type recentPage struct {
	id     string
	isCash bool
}

const maxRecent = 50

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
	return c.createPage(c.dbID, false, expenseProps(date, name, amount, categoryID, description))
}

// CreateCashExpense adds an item paid in cash, linked to the withdrawal page it came from.
func (c *Client) CreateCashExpense(date time.Time, name string, amount float64, categoryID, description, withdrawalID string) error {
	props := expenseProps(date, name, amount, categoryID, description)
	props["Withdrawal"] = map[string]interface{}{
		"relation": []map[string]string{{"id": withdrawalID}},
	}
	return c.createPage(c.cashDBID, true, props)
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

func (c *Client) createPage(dbID string, isCash bool, props map[string]interface{}) error {
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

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err == nil && created.ID != "" {
		c.remember(created.ID, isCash)
	}
	return nil
}

func (c *Client) remember(id string, isCash bool) {
	c.recentMu.Lock()
	defer c.recentMu.Unlock()
	c.recent = append(c.recent, recentPage{id, isCash})
	if len(c.recent) > maxRecent {
		c.recent = c.recent[len(c.recent)-maxRecent:]
	}
}

func (c *Client) forget(id string) {
	c.recentMu.Lock()
	defer c.recentMu.Unlock()
	for i := len(c.recent) - 1; i >= 0; i-- {
		if c.recent[i].id == id {
			c.recent = append(c.recent[:i], c.recent[i+1:]...)
		}
	}
}

// newestRemembered returns the most recently created page that still exists,
// dropping any that were deleted in Notion since.
func (c *Client) newestRemembered() (*map[string]interface{}, bool, error) {
	for {
		c.recentMu.Lock()
		if len(c.recent) == 0 {
			c.recentMu.Unlock()
			return nil, false, nil
		}
		top := c.recent[len(c.recent)-1]
		c.recentMu.Unlock()

		page, err := c.getPage(top.id)
		if err != nil {
			return nil, false, err
		}
		if page != nil {
			return page, top.isCash, nil
		}
		c.forget(top.id)
	}
}

// getPage fetches a page. It returns nil if the page is gone or in the trash.
func (c *Client) getPage(id string) (*map[string]interface{}, error) {
	req, err := http.NewRequest("GET", APIBaseURL+"/pages/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("notion get page failed: %s", resp.Status)
	}

	var page map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if archived, _ := page["archived"].(bool); archived {
		return nil, nil
	}
	if trashed, _ := page["in_trash"].(bool); trashed {
		return nil, nil
	}
	return &page, nil
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
