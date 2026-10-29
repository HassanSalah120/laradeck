package mail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	input := `<div>
		<h1>Welcome!</h1>
		<script>alert("xss")</script>
		<img src="https://tracker.example.com/pixel.png" alt="tracker" />
		<a href="https://example.com" onclick="steal()">Click me</a>
		<iframe src="https://evil.com"></iframe>
		<svg/onload=globalThis.__codex_mail_probe=1>
		<img src=x onerror=alert(1)>
		<a href="javascript:alert(document.cookie)">Malicious Link</a>
		<img src="//protocol-relative.example.com/track.gif" />
		<style>body { background: url('http://evil.com/css.png'); }</style>
	</div>`

	safe, hasRemote := SanitizeHTML(input)
	if !hasRemote {
		t.Errorf("expected hasRemote to be true")
	}
	if strings.Contains(safe, "<script") {
		t.Errorf("script tag not removed")
	}
	if strings.Contains(safe, "<iframe") {
		t.Errorf("iframe tag not removed")
	}
	if strings.Contains(safe, "<svg") {
		t.Errorf("svg tag not removed: %s", safe)
	}
	if strings.Contains(safe, "globalThis") || strings.Contains(safe, "__codex_mail_probe") {
		t.Errorf("svg onload payload survived: %s", safe)
	}
	if strings.Contains(safe, "onclick") || strings.Contains(safe, "onerror") || strings.Contains(safe, "onload") {
		t.Errorf("event handler attribute not removed: %s", safe)
	}
	if strings.Contains(safe, "javascript:") {
		t.Errorf("javascript: URI not removed: %s", safe)
	}
	if strings.Contains(safe, "<style") {
		t.Errorf("style tag not removed: %s", safe)
	}
	if strings.Contains(safe, ` src="https://tracker.example.com`) {
		t.Errorf("remote image src should be blocked")
	}
	if !strings.Contains(safe, `data-blocked-src="https://tracker.example.com`) {
		t.Errorf("expected data-blocked-src attribute")
	}
	if !strings.Contains(safe, `data-blocked-src="//protocol-relative.example.com`) {
		t.Errorf("expected protocol-relative image to be blocked")
	}
}

func TestSanitizeHTML_OrdinaryMarkupNoPanic(t *testing.T) {
	// Tests ordinary email layouts including <center>, <section>, <font>, and unknown elements.
	// Verifies node reparenting does not panic.
	cases := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "center tag",
			input:    "<center>Hello World</center>",
			contains: "Hello World",
		},
		{
			name:     "section with paragraph",
			input:    "<section><p>Section paragraph</p></section>",
			contains: "Section paragraph",
		},
		{
			name:     "font tag with color",
			input:    `<font color="red" size="3">Styled Text</font>`,
			contains: "Styled Text",
		},
		{
			name:     "unknown custom tag safely unwrapped",
			input:    "<custom-widget><p>Nested Safe Content</p></custom-widget>",
			contains: "<p>Nested Safe Content</p>",
		},
		{
			name:     "table with center and bold",
			input:    `<table><tr><td><center><b>Important</b></center></td></tr></table>`,
			contains: "Important",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("SanitizeHTML panicked on %q: %v", tc.input, r)
				}
			}()

			cleaned, _ := SanitizeHTML(tc.input)
			if !strings.Contains(cleaned, tc.contains) {
				t.Errorf("expected cleaned output to contain %q, got: %s", tc.contains, cleaned)
			}
		})
	}
}

func TestMailpitClientMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/info" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"v1.18.0"}`))
			return
		}
		if r.URL.Path == "/api/v1/messages" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total": 1,
				"unread": 1,
				"messages": [{
					"ID": "msg123",
					"From": {"Address": "sender@test.local", "Name": "Sender"},
					"To": [{"Address": "recipient@test.local", "Name": "Recipient"}],
					"Subject": "Test Email",
					"Snippet": "Hello from Laravel",
					"Created": "2026-10-16T01:00:00Z",
					"Size": 1024,
					"Attachments": 0,
					"Read": false
				}]
			}`))
			return
		}
		if r.URL.Path == "/api/v1/message/msg123/headers" {
			w.Write([]byte(`{"Subject":["Test Email"]}`))
			return
		}
		if r.URL.Path == "/api/v1/message/msg123" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"ID": "msg123",
				"From": {"Address": "sender@test.local"},
				"To": [{"Address": "recipient@test.local"}],
				"Subject": "Test Email",
				"Date": "2026-10-16T01:00:00Z",
				"Text": "Hello from Laravel",
				"HTML": "<p>Hello from Laravel</p>",
				
				"Attachments": []
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewMailpitClient(server.URL)
	ctx := context.Background()

	healthy, err := client.CheckHealth(ctx)
	if err != nil || !healthy {
		t.Fatalf("health check failed: %v", err)
	}

	box, err := client.GetMessages(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if box.Total != 1 || len(box.Messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", box)
	}
	if box.Messages[0].Subject != "Test Email" {
		t.Errorf("expected Subject 'Test Email', got %s", box.Messages[0].Subject)
	}

	msg, err := client.GetMessage(ctx, "msg123")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg.Subject != "Test Email" || msg.Text != "Hello from Laravel" {
		t.Errorf("unexpected msg content: %+v", msg)
	}
}

func TestSearchUsesSearchEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" || r.URL.Query().Get("query") != "subject:invoice" {
			t.Errorf("wrong search request: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"total":0,"unread":0,"messages":[]}`))
	}))
	defer server.Close()
	if _, err := NewMailpitClient(server.URL).GetMessages(context.Background(), "subject:invoice", 10, 0); err != nil {
		t.Fatal(err)
	}
}
func TestDeleteDoesNotReportHTTPFailureAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer server.Close()
	client := NewMailpitClient(server.URL)
	if client.DeleteAll(context.Background()) == nil {
		t.Fatal("delete all hid HTTP 500")
	}
	if client.DeleteMessage(context.Background(), "id") == nil {
		t.Fatal("delete message hid HTTP 500")
	}
	if client.DeleteMessage(context.Background(), "") == nil {
		t.Fatal("empty ID accepted")
	}
}
