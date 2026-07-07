package hatena

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var (
	// ErrUnauthorized is returned when the API rejects the request with 401.
	ErrUnauthorized = errors.New("authentication failed (401)")
	// ErrNotFound is returned when the API responds with 404.
	ErrNotFound = errors.New("entry not found (404)")
)

// Client is a Hatena Blog AtomPub API client.
type Client struct {
	hatenaID    string
	blogID      string
	apiKey      string
	baseURL     string
	fotolifeURL string
	http        *http.Client
}

// NewClient returns a Client configured for the given credentials.
func NewClient(hatenaID, blogID, apiKey string, timeoutSecond int) *Client {
	return &Client{
		hatenaID:    hatenaID,
		blogID:      blogID,
		apiKey:      apiKey,
		baseURL:     "https://blog.hatena.ne.jp",
		fotolifeURL: "https://f.hatena.ne.jp/atom/post",
		http: &http.Client{
			Timeout: time.Duration(timeoutSecond) * time.Second,
			// X-WSSE is not among the headers net/http strips on cross-host
			// redirects, so following one would leak the credential digest.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Host != via[0].URL.Host {
					return fmt.Errorf("refusing cross-host redirect to %s", req.URL.Host)
				}
				return nil
			},
		},
	}
}

// SetBaseURL overrides the base URL, intended for testing.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// SetFotolifeURL overrides the Fotolife API endpoint, intended for testing.
func (c *Client) SetFotolifeURL(url string) {
	c.fotolifeURL = url
}

func (c *Client) collectionURL() string {
	return fmt.Sprintf("%s/%s/%s/atom/entry", c.baseURL, c.hatenaID, c.blogID)
}

// validateEditURL rejects edit URLs whose scheme or host differ from the base URL.
// Edit URLs come from local frontmatter, which must not be able to direct
// WSSE-authenticated requests at arbitrary hosts.
func (c *Client) validateEditURL(editURL string) error {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL %s: %w", c.baseURL, err)
	}
	u, err := url.Parse(editURL)
	if err != nil {
		return fmt.Errorf("parse edit URL %s: %w", editURL, err)
	}
	if u.Scheme != base.Scheme || u.Host != base.Host {
		return fmt.Errorf("edit URL %s does not match API host %s", editURL, base.Host)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	wsseHeader, err := GenerateWSSEHeader(c.hatenaID, c.apiKey)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-WSSE", wsseHeader)
	req.Header.Set("Authorization", "WSSE profile=\"UsernameToken\"")
	if body != nil {
		// Both Blog API and Fotolife API are Atom Publishing Protocol endpoints,
		// so application/atom+xml is the correct Content-Type for all request bodies.
		req.Header.Set("Content-Type", "application/atom+xml")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, url, err)
	}
	return resp, nil
}

// request performs an authenticated request and returns the response body
// and status code after rejecting non-success statuses.
func (c *Client) request(ctx context.Context, method, url string, body []byte) (data []byte, statusCode int, err error) {
	resp, err := c.do(ctx, method, url, body)
	if err != nil {
		return nil, 0, err
	}
	data, err = readBody(resp)
	if err != nil {
		return nil, 0, err
	}
	if err := checkStatus(resp, data); err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
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
	case http.StatusOK, http.StatusCreated, http.StatusNoContent: // NoContent is used by DeleteEntry
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, data)
	}
}

// ListEntries fetches entries from the blog, following pagination.
// maxPages limits the number of pages fetched; 0 means no limit.
func (c *Client) ListEntries(ctx context.Context, maxPages int) ([]*Entry, error) {
	url := c.collectionURL()
	var all []*Entry
	for page := 1; url != ""; page++ {
		data, _, err := c.request(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		entries, nextURL, err := parseFeed(data)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
		if maxPages > 0 && page >= maxPages {
			break
		}
		url = nextURL
	}
	return all, nil
}

// GetEntry fetches a single entry by its edit URL.
func (c *Client) GetEntry(ctx context.Context, editURL string) (*Entry, error) {
	if err := c.validateEditURL(editURL); err != nil {
		return nil, err
	}
	data, _, err := c.request(ctx, http.MethodGet, editURL, nil)
	if err != nil {
		return nil, err
	}
	return parseEntry(data)
}

// CreateEntry posts a new entry and returns the created entry.
func (c *Client) CreateEntry(ctx context.Context, e *Entry) (*Entry, error) {
	body, err := marshalEntry(e)
	if err != nil {
		return nil, err
	}
	data, _, err := c.request(ctx, http.MethodPost, c.collectionURL(), body)
	if err != nil {
		return nil, err
	}
	return parseEntry(data)
}

// UpdateEntry updates an existing entry via PUT to its edit URL.
func (c *Client) UpdateEntry(ctx context.Context, editURL string, e *Entry) (*Entry, error) {
	if err := c.validateEditURL(editURL); err != nil {
		return nil, err
	}
	body, err := marshalEntry(e)
	if err != nil {
		return nil, err
	}
	data, statusCode, err := c.request(ctx, http.MethodPut, editURL, body)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusNoContent {
		// Server accepted the update but returned no body; use the sent entry.
		return e, nil
	}
	return parseEntry(data)
}

// DeleteEntry deletes the remote entry identified by editURL.
func (c *Client) DeleteEntry(ctx context.Context, editURL string) error {
	if err := c.validateEditURL(editURL); err != nil {
		return err
	}
	_, _, err := c.request(ctx, http.MethodDelete, editURL, nil)
	return err
}
