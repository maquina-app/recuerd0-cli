package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/maquina/recuerd0-cli/internal/errors"
)

// Client implements the API interface for making HTTP requests to the Recuerd0 API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Verbose    bool
}

// New creates a new API client.
func New(baseURL, token string, verbose bool) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Verbose: verbose,
	}
}

func (c *Client) buildURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return c.BaseURL + path
}

func (c *Client) doRequest(method, path string, body interface{}) (*APIResponse, error) {
	url := c.buildURL(path)

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, errors.NewError(fmt.Sprintf("marshaling request body: %v", err))
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, errors.NewNetworkError(fmt.Sprintf("creating request: %v", err))
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "--> %s %s\n", method, url)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, errors.NewNetworkError(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewNetworkError(fmt.Sprintf("reading response: %v", err))
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "<-- %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	apiResp := &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Location:   resp.Header.Get("Location"),
		LinkNext:   parseLinkNext(resp.Header.Get("Link")),
	}

	// Parse JSON body
	if len(respBody) > 0 {
		var data interface{}
		if err := json.Unmarshal(respBody, &data); err == nil {
			apiResp.Data = data
		}
	}

	// Handle error status codes
	if resp.StatusCode >= 400 {
		msg := extractErrorMessage(apiResp.Data, respBody)
		return nil, errors.FromHTTPStatus(resp.StatusCode, msg)
	}

	return apiResp, nil
}

func (c *Client) Get(path string) (*APIResponse, error) {
	return c.doRequest("GET", path, nil)
}

func (c *Client) Post(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest("POST", path, body)
}

func (c *Client) Patch(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest("PATCH", path, body)
}

func (c *Client) Delete(path string) (*APIResponse, error) {
	return c.doRequest("DELETE", path, nil)
}

func (c *Client) GetWithPagination(path string) (*APIResponse, error) {
	return c.Get(path)
}

// parseLinkNext extracts the "next" URL from a Link header (RFC 5988).
func parseLinkNext(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	re := regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)
	matches := re.FindStringSubmatch(linkHeader)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractErrorMessage tries to pull a message from the API error response.
func extractErrorMessage(data interface{}, raw []byte) string {
	if m, ok := data.(map[string]interface{}); ok {
		// {"error": "message"}
		if msg, ok := m["error"].(string); ok {
			return msg
		}
		// {"error": {"code": "...", "message": "...", "details": {"field": ["msg"]}}}
		if errObj, ok := m["error"].(map[string]interface{}); ok {
			if details, ok := errObj["details"].(map[string]interface{}); ok {
				var parts []string
				for field, val := range details {
					if msgs, ok := val.([]interface{}); ok {
						for _, msg := range msgs {
							if s, ok := msg.(string); ok {
								parts = append(parts, field+" "+s)
							}
						}
					}
				}
				if len(parts) > 0 {
					return strings.Join(parts, "; ")
				}
			}
			if msg, ok := errObj["message"].(string); ok {
				return msg
			}
		}
		// {"message": "..."}
		if msg, ok := m["message"].(string); ok {
			return msg
		}
		// {"errors": {"name": ["can't be blank"], ...}} (Rails-style)
		if errs, ok := m["errors"].(map[string]interface{}); ok {
			var parts []string
			for field, val := range errs {
				if msgs, ok := val.([]interface{}); ok {
					for _, msg := range msgs {
						if s, ok := msg.(string); ok {
							parts = append(parts, field+" "+s)
						}
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
		// {"errors": ["message", ...]}
		if errs, ok := m["errors"].([]interface{}); ok && len(errs) > 0 {
			if s, ok := errs[0].(string); ok {
				return s
			}
		}
	}
	if len(raw) > 0 && len(raw) < 200 {
		return string(raw)
	}
	return "unknown error"
}
