package frontier

import (
	"regexp"
	"strings"

	"llama-wrangler/internal/config"
)

type CheckResult struct {
	Allowed         bool     `json:"allowed"`
	SecretsDetected bool     `json:"secrets_detected"`
	Redacted        bool     `json:"redacted"`
	Reasons         []string `json:"reasons"`
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_\-.]{20,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(aws_access_key_id|aws_secret_access_key|github_token|splunk[_-]?token)\s*[:=]`),
	regexp.MustCompile(`(?i)password\s*[:=]\s*[^ \n]+`),
}

func Check(cfg config.FrontierConfig, text string) CheckResult {
	result := CheckResult{Allowed: cfg.Enabled && !cfg.LocalOnly}
	if !cfg.Enabled {
		result.Reasons = append(result.Reasons, "frontier_disabled")
	}
	if cfg.LocalOnly {
		result.Reasons = append(result.Reasons, "local_only")
	}
	for _, pattern := range secretPatterns {
		if pattern.MatchString(text) {
			result.SecretsDetected = true
			result.Allowed = false
			result.Reasons = append(result.Reasons, "secrets_detected")
			break
		}
	}
	if strings.Contains(strings.ToLower(strings.Join(cfg.RequireApprovalFor, " ")), "source_code") {
		result.Reasons = append(result.Reasons, "approval_required")
		result.Allowed = false
	}
	return result
}
