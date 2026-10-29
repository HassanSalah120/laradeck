package remote

import (
	"fmt"
	"regexp"
	"strings"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(migrate:fresh|migrate:reset|migrate:rollback)\b`),
	regexp.MustCompile(`(?i)\b(db:seed|db:wipe)\b`),
	regexp.MustCompile(`(?i)\b(rm\s+-rf|rmdir\s+/s)\b`),
	regexp.MustCompile(`(?i)\b(drop\s+table|drop\s+database|truncate\s+table)\b`),
	regexp.MustCompile(`(?i)\b(delete\s+from\s+[a-zA-Z0-9_]+\s*;?\s*$)`),
}

type SafetyAssessment struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason"`
	RequiresToken bool   `json:"requiresToken"`
	ExpectedToken string `json:"expectedToken,omitempty"`
}

// AssessCommandSafety evaluates whether a command is safe to execute on a given remote profile
func AssessCommandSafety(profile RemoteProfile, command string, confirmationToken string) SafetyAssessment {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return SafetyAssessment{Allowed: false, Reason: "Empty command"}
	}

	isProduction := strings.ToLower(profile.Environment) == "production"

	for _, pat := range dangerousPatterns {
		if pat.MatchString(trimmed) {
			match := pat.FindString(trimmed)

			// 1. If ReadOnly profile is set, block any non-read commands unconditionally
			if profile.ReadOnly {
				return SafetyAssessment{
					Allowed: false,
					Reason:  fmt.Sprintf("Profile '%s' is locked in Read-Only Mode. Destructive operation '%s' is prohibited.", profile.Name, match),
				}
			}

			// 2. If Environment is production, enforce explicit token
			if isProduction {
				expected := "CONFIRM-PRODUCTION-MUTATION"
				if confirmationToken != expected {
					return SafetyAssessment{
						Allowed:       false,
						Reason:        fmt.Sprintf("Potentially destructive command '%s' detected on PRODUCTION environment.", match),
						RequiresToken: true,
						ExpectedToken: expected,
					}
				}
			}
		}
	}

	return SafetyAssessment{Allowed: true, Reason: "Command passed safety interlock checks"}
}
