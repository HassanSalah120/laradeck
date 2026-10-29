package storage

import (
	"regexp"
	"strings"
)

var (
	bearerRegex     = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9\-\._~\+\/]+=*`)
	kvSecretRegex   = regexp.MustCompile(`(?i)\b((?:[a-z0-9]+_)*(?:password|passwd|pwd|secret|token|api_?key|auth|access_?token|client_?secret|private_?key|ssh_?key|cookie|session|signature|authorization|key))\s*[:=]\s*(?:'[^']*'|"[^"]*"|[^\s,;]+)`)
	jsonSecretRegex = regexp.MustCompile(`(?i)"((?:[a-z0-9]+_)*(?:password|token|secret|api_?key|auth|key|access_token|client_secret|authorization|cookie|session))"\s*:\s*("(?:\\"|[^"])*"|[^\s,}\]]+)`)
	urlAuthRegex    = regexp.MustCompile(`([a-zA-Z]+://[^:\s]+):([^@\s]+)@`)
	knownKeyRegex   = regexp.MustCompile(`(?i)\b(AKIA[0-9A-Z]{16}|ghp_[0-9a-zA-Z]{36}|gho_[0-9a-zA-Z]{36}|github_pat_[0-9a-zA-Z_]{82}|sk_live_[0-9a-zA-Z]{24})\b`)
)

// RedactSensitive applies structured secret minimization and redaction to text before persistence or export.
func RedactSensitive(text string) string {
	if text == "" {
		return ""
	}

	// 1. Redact URL Basic Auth
	s := urlAuthRegex.ReplaceAllString(text, "$1:[redacted]@")

	// 2. Redact Bearer tokens
	s = bearerRegex.ReplaceAllString(s, "$1[redacted]")

	// 3. Redact Key-Value secret lines
	s = kvSecretRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		sep := "="
		if len(parts) == 1 {
			parts = strings.SplitN(match, ":", 2)
			sep = ":"
		}
		if len(parts) == 2 {
			return parts[0] + sep + "[redacted]"
		}
		return match
	})

	// 4. Redact JSON field values
	s = jsonSecretRegex.ReplaceAllString(s, `"$1":"[redacted]"`)

	// 5. Redact known vendor secret patterns (AWS, GitHub, Stripe)
	s = knownKeyRegex.ReplaceAllString(s, "[redacted]")

	// 6. Max length minimization (prevent runaway log payloads from flooding DB)
	if len(s) > 32000 {
		s = s[:32000] + " …[truncated]"
	}

	return s
}
