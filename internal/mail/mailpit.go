package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MailMessageSummary struct {
	ID          string   `json:"id"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	Subject     string   `json:"subject"`
	Snippet     string   `json:"snippet"`
	CreatedAt   string   `json:"createdAt"`
	Size        int      `json:"size"`
	Attachments int      `json:"attachments"`
	Read        bool     `json:"read"`
}

type MailAttachment struct {
	PartID      string `json:"partId"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

type MailMessageDetail struct {
	ID          string            `json:"id"`
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Cc          []string          `json:"cc"`
	Bcc         []string          `json:"bcc"`
	Subject     string            `json:"subject"`
	CreatedAt   string            `json:"createdAt"`
	Text        string            `json:"text"`
	HTML        string            `json:"html"`
	SafeHTML    string            `json:"safeHtml"`
	HasRemote   bool              `json:"hasRemoteImages"`
	Headers     map[string]string `json:"headers"`
	Attachments []MailAttachment  `json:"attachments"`
}

type MailboxResult struct {
	Total    int                  `json:"total"`
	Unread   int                  `json:"unread"`
	Messages []MailMessageSummary `json:"messages"`
}

type MailpitClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewMailpitClient(baseURL string) *MailpitClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8025"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &MailpitClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckHealth verifies connection to Mailpit.
func (c *MailpitClient) CheckHealth(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/info", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// GetMessages lists messages with optional query search, offset, and limit.
func (c *MailpitClient) GetMessages(ctx context.Context, search string, limit, start int) (*MailboxResult, error) {
	if limit <= 0 {
		limit = 50
	}
	endpoint := fmt.Sprintf("%s/api/v1/messages?limit=%d&start=%d", c.baseURL, limit, start)
	if search != "" {
		endpoint = fmt.Sprintf("%s/api/v1/search?limit=%d&start=%d&query=%s", c.baseURL, limit, start, url.QueryEscape(search))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mailpit returned HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Total    int `json:"total"`
		Unread   int `json:"unread"`
		Messages []struct {
			ID   string `json:"ID"`
			From struct {
				Address string `json:"Address"`
				Name    string `json:"Name"`
			} `json:"From"`
			To []struct {
				Address string `json:"Address"`
				Name    string `json:"Name"`
			} `json:"To"`
			Subject     string    `json:"Subject"`
			Snippet     string    `json:"Snippet"`
			Created     time.Time `json:"Created"`
			Size        int       `json:"Size"`
			Attachments int       `json:"Attachments"`
			Read        bool      `json:"Read"`
		} `json:"messages"`
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16*1024*1024 {
		return nil, errors.New("Mailpit response exceeds 16 MiB limit")
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	res := &MailboxResult{
		Total:    raw.Total,
		Unread:   raw.Unread,
		Messages: make([]MailMessageSummary, 0, len(raw.Messages)),
	}

	for _, m := range raw.Messages {
		fromStr := m.From.Address
		if m.From.Name != "" {
			fromStr = fmt.Sprintf("%s <%s>", m.From.Name, m.From.Address)
		}
		var toList []string
		for _, t := range m.To {
			if t.Name != "" {
				toList = append(toList, fmt.Sprintf("%s <%s>", t.Name, t.Address))
			} else {
				toList = append(toList, t.Address)
			}
		}
		res.Messages = append(res.Messages, MailMessageSummary{
			ID:          m.ID,
			From:        fromStr,
			To:          toList,
			Subject:     m.Subject,
			Snippet:     m.Snippet,
			CreatedAt:   m.Created.Format(time.RFC3339),
			Size:        m.Size,
			Attachments: m.Attachments,
			Read:        m.Read,
		})
	}

	return res, nil
}

// GetMessage returns detailed message information with sanitized HTML.
func (c *MailpitClient) GetMessage(ctx context.Context, id string) (*MailMessageDetail, error) {
	if id == "" {
		return nil, errors.New("Message ID required")
	}
	endpoint := fmt.Sprintf("%s/api/v1/message/%s", c.baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mailpit returned HTTP %d", resp.StatusCode)
	}

	var raw struct {
		ID   string `json:"ID"`
		From struct {
			Address string `json:"Address"`
			Name    string `json:"Name"`
		} `json:"From"`
		To []struct {
			Address string `json:"Address"`
			Name    string `json:"Name"`
		} `json:"To"`
		Cc []struct {
			Address string `json:"Address"`
			Name    string `json:"Name"`
		} `json:"Cc"`
		Bcc []struct {
			Address string `json:"Address"`
			Name    string `json:"Name"`
		} `json:"Bcc"`
		Subject     string              `json:"Subject"`
		Created     time.Time           `json:"Created"`
		Date        time.Time           `json:"Date"`
		Text        string              `json:"Text"`
		HTML        string              `json:"HTML"`
		Headers     map[string][]string `json:"Headers"`
		Attachments []struct {
			PartID      string `json:"PartID"`
			FileName    string `json:"FileName"`
			ContentType string `json:"ContentType"`
			Size        int    `json:"Size"`
		} `json:"Attachments"`
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16*1024*1024 {
		return nil, errors.New("Mailpit response exceeds 16 MiB limit")
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	fromStr := raw.From.Address
	if raw.From.Name != "" {
		fromStr = fmt.Sprintf("%s <%s>", raw.From.Name, raw.From.Address)
	}
	toList := make([]string, 0, len(raw.To))
	for _, t := range raw.To {
		toList = append(toList, t.Address)
	}
	ccList := make([]string, 0, len(raw.Cc))
	for _, t := range raw.Cc {
		ccList = append(ccList, t.Address)
	}
	bccList := make([]string, 0, len(raw.Bcc))
	for _, t := range raw.Bcc {
		bccList = append(bccList, t.Address)
	}

	if raw.Date.IsZero() {
		raw.Date = raw.Created
	}
	headersReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/headers", nil)
	if err != nil {
		return nil, err
	}
	headersResp, err := c.httpClient.Do(headersReq)
	if err != nil {
		return nil, err
	}
	defer headersResp.Body.Close()
	if headersResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mailpit headers returned HTTP %d", headersResp.StatusCode)
	}
	headerBytes, err := io.ReadAll(io.LimitReader(headersResp.Body, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(headerBytes) > 1024*1024 {
		return nil, errors.New("Mailpit headers exceed 1 MiB")
	}
	if err := json.Unmarshal(headerBytes, &raw.Headers); err != nil {
		return nil, fmt.Errorf("Mailpit headers: %w", err)
	}
	hdrs := make(map[string]string)
	for k, v := range raw.Headers {
		if len(v) > 0 {
			hdrs[k] = v[0]
		}
	}

	var atts []MailAttachment
	for _, a := range raw.Attachments {
		atts = append(atts, MailAttachment{
			PartID:      a.PartID,
			FileName:    a.FileName,
			ContentType: a.ContentType,
			Size:        a.Size,
		})
	}

	safeHTML, hasRemote := SanitizeHTML(raw.HTML)

	return &MailMessageDetail{
		ID:          raw.ID,
		From:        fromStr,
		To:          toList,
		Cc:          ccList,
		Bcc:         bccList,
		Subject:     raw.Subject,
		CreatedAt:   raw.Date.Format(time.RFC3339),
		Text:        raw.Text,
		HTML:        raw.HTML,
		SafeHTML:    safeHTML,
		HasRemote:   hasRemote,
		Headers:     hdrs,
		Attachments: atts,
	}, nil
}

// DeleteAll deletes all messages in Mailpit.
func (c *MailpitClient) DeleteAll(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/messages", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Mailpit returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// DeleteMessage deletes a single message.
func (c *MailpitClient) DeleteMessage(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("Message ID required")
	}
	payload, _ := json.Marshal(map[string]any{"IDs": []string{id}})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Mailpit returned HTTP %d", resp.StatusCode)
	}
	return nil
}
