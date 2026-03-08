package chatbridge

import (
	"bytes"
	"strings"
	"text/template"
)

func DetectIncidentKind(cfg TriggerConfig, text string) (IncidentKind, bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return "", false
	}

	if len(cfg.RequirePrefixes) > 0 && !hasAnyPrefix(normalized, cfg.RequirePrefixes) {
		return "", false
	}

	if containsAny(normalized, cfg.SolaceKeywords) {
		return IncidentSolace, true
	}
	if containsAny(normalized, cfg.KubernetesKeywords) {
		return IncidentKubernetes, true
	}
	return "", false
}

func RenderPrompt(tmpl string, req InvestigationRequest) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		return req.MessageText, nil
	}
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, req); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		trimmed := strings.ToLower(strings.TrimSpace(term))
		if trimmed == "" {
			continue
		}
		if strings.Contains(text, trimmed) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(text string, prefixes []string) bool {
	for _, prefix := range prefixes {
		trimmed := strings.ToLower(strings.TrimSpace(prefix))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(text, trimmed) {
			return true
		}
	}
	return false
}

func isAllowedSpace(cfg TriggerConfig, spaceName string) bool {
	if len(cfg.AllowSpaces) == 0 {
		return true
	}
	for _, allowed := range cfg.AllowSpaces {
		if strings.TrimSpace(allowed) == strings.TrimSpace(spaceName) {
			return true
		}
	}
	return false
}
