package hatena

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Hatena Blog AtomPub API client.
type Client struct {
	hatenaID string
	blogID   string
	apiKey   string
	baseURL  string
	http     *http.Client
}

// NewClient returns a Client configured for the given credentials.
func NewClient(hatenaID, blogID, apiKey string) *Client {
	return &Client{
		hatenaID: hatenaID,
		blogID:   blogID,
		apiKey:   apiKey,
		baseURL:  "https://blog.hatena.ne.jp",
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// SetBaseURL overrides the base URL, intended for testing.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

func (c *Client) collectionURL() string {
	return fmt.Sprintf("%s/%s/%s/atom/entry", c.baseURL, c.hatenaID, c.blogID)
}

func (c *Client) do(method, url string, body []byte) (*http.Response, error) {
	wsseHeader, err := GenerateWSSEHeader(c.hatenaID, c.apiKey)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-WSSE", wsseHeader)
	req.Header.Set("Authorization", "WSSE profile=\"UsernameToken\"")
	if body != nil {
		req.Header.Set("Content-Type", "application/atom+xml")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, fmt.Errorf("http %s %s: %w", method, url, err)
	}
	return resp, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return data, nil
}

func checkStatus(resp *http.Response, data []byte) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed (401)")
	case http.StatusNotFound:
		return fmt.Errorf("entry not found (404)")
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, data)
	}
}

// ListEntries fetches all entries from the blog, following pagination.
func (c *Client) ListEntries() ([]*Entry, error) {
	url := c.collectionURL()
	var all []*Entry
	for url != "" {
		resp, err := c.do(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		data, err := readBody(resp)
		if err != nil {
			return nil, err
		}
		if err := checkStatus(resp, data); err != nil {
			return nil, err
		}
		entries, nextURL, err := parseFeed(data)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
		url = nextURL
	}
	return all, nil
}

// GetEntry fetches a single entry by its edit URL.
func (c *Client) GetEntry(editURL string) (*Entry, error) {
	resp, err := c.do(http.MethodGet, editURL, nil)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp, data); err != nil {
		return nil, err
	}
	return parseEntry(data)
}

// CreateEntry posts a new entry and returns the created entry.
func (c *Client) CreateEntry(e *Entry) (*Entry, error) {
	body, err := marshalEntry(e)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(http.MethodPost, c.collectionURL(), body)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp, data); err != nil {
		return nil, err
	}
	return parseEntry(data)
}

// UpdateEntry updates an existing entry via PUT to its edit URL.
func (c *Client) UpdateEntry(editURL string, e *Entry) (*Entry, error) {
	body, err := marshalEntry(e)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(http.MethodPut, editURL, body)
	if err != nil {
		return nil, err
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp, data); err != nil {
		return nil, err
	}
	return parseEntry(data)
}
