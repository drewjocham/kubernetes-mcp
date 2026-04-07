package security

import (
	"regexp"
	"strings"

	"kube-watcher/watcher/internal/events"
)

// SensitiveFieldPatterns defines regex patterns for sensitive data in field names
var SensitiveFieldPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|api[_-]?token)`),
	regexp.MustCompile(`(?i)(secret|password|passwd|pwd|credential|token|auth)`),
	regexp.MustCompile(`(?i)(private[_-]?key|ssh[_-]?key|rsa[_-]?key)`),
	regexp.MustCompile(`(?i)(access[_-]?token|refresh[_-]?token|bearer[_-]?token)`),
	regexp.MustCompile(`(?i)(license[_-]?key|licence[_-]?key)`),
}

// SensitiveValuePatterns defines regex patterns for sensitive data in values
var SensitiveValuePatterns = []*regexp.Regexp{
	// JWT tokens
	regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
	// API keys (hex, base64, alphanumeric with dashes)
	regexp.MustCompile(`(?i)(sk_|pk_)[a-zA-Z0-9_-]{20,}`),
	regexp.MustCompile(`[a-f0-9]{32,}`),      // hex strings
	regexp.MustCompile(`[A-Za-z0-9+/]{40,}`), // base64-like
	// Common password patterns
	regexp.MustCompile(`(?i)password\s*[:=]\s*[^\s]{6,}`),
	regexp.MustCompile(`(?i)pass\s*[:=]\s*[^\s]{6,}`),
}

// SensitiveFieldDenyList defines exact field names that should be redacted
var SensitiveFieldDenyList = map[string]bool{
	// Environment variable references
	"env":       true,
	"envFrom":   true,
	"valueFrom": true,
	// Secret references
	"secretRef":    true,
	"secretKeyRef": true,
	"secrets":      true,
	// ConfigMap references
	"configMapRef":    true,
	"configMapKeyRef": true,
	// Container fields
	"imagePullSecrets": true,
	// Service account fields
	"serviceAccountToken": true,
	// Volume fields
	"secret":                    true,
	"csi.storage.k8s.io/secret": true,
}

// Sanitizer provides methods to sanitize sensitive data from events
type Sanitizer struct {
	enabled bool
}

// NewSanitizer creates a new sanitizer with optional configuration
func NewSanitizer(enabled bool) *Sanitizer {
	return &Sanitizer{
		enabled: enabled,
	}
}

// SanitizeEvent removes or masks sensitive data from a ResourceEvent
func (s *Sanitizer) SanitizeEvent(event *events.ResourceEvent) *events.ResourceEvent {
	if !s.enabled || event.Object == nil {
		return event
	}

	// Create a copy to avoid modifying the original
	sanitized := *event
	sanitized.Object = s.sanitizeMap(event.Object, "")
	// Clear Raw field to prevent sensitive data leakage through runtime.Object serialization
	sanitized.Raw = nil

	return &sanitized
}

// sanitizeMap recursively sanitizes a map[string]interface{}
func (s *Sanitizer) sanitizeMap(m map[string]interface{}, path string) map[string]interface{} {
	if m == nil {
		return nil
	}

	sanitized := make(map[string]interface{}, len(m))
	for key, value := range m {
		fullPath := path
		if fullPath != "" {
			fullPath += "."
		}
		fullPath += key

		sanitized[key] = s.sanitizeValue(value, fullPath, key)
	}

	return sanitized
}

// sanitizeValue sanitizes a value based on its type and field path
func (s *Sanitizer) sanitizeValue(value interface{}, path, fieldName string) interface{} {
	// Special handling for env arrays inside containers - they get fine-grained sanitization
	isEnvInContainers := fieldName == "env" && strings.Contains(path, ".containers")

	// If field is sensitive and not an env array inside containers, redact entire value
	if s.isSensitiveField(fieldName) && !isEnvInContainers {
		return "[REDACTED]"
	}

	switch v := value.(type) {
	case map[string]interface{}:
		return s.sanitizeMap(v, path)
	case []interface{}:
		return s.sanitizeSlice(v, path, fieldName)
	case string:
		return s.sanitizeString(v, fieldName)
	default:
		return v
	}
}

// sanitizeSlice sanitizes a slice of values
func (s *Sanitizer) sanitizeSlice(slice []interface{}, path, fieldName string) []interface{} {
	if slice == nil {
		return nil
	}

	// Special handling for env arrays in containers
	if strings.Contains(path, ".containers") && fieldName == "env" {
		return s.sanitizeEnvSlice(slice)
	}

	sanitized := make([]interface{}, len(slice))
	for i, item := range slice {
		sanitized[i] = s.sanitizeValue(item, path+"["+string(rune(i))+"]", fieldName)
	}
	return sanitized
}

// sanitizeEnvSlice sanitizes environment variable arrays
func (s *Sanitizer) sanitizeEnvSlice(envSlice []interface{}) []interface{} {
	sanitized := make([]interface{}, len(envSlice))
	for i, item := range envSlice {
		if envMap, ok := item.(map[string]interface{}); ok {
			sanitizedEnv := make(map[string]interface{}, len(envMap))
			for key, value := range envMap {
				switch key {
				case "name":
					sanitizedEnv[key] = value
				case "value":
					if _, ok := value.(string); ok && s.isSensitiveEnvVar(envMap) {
						sanitizedEnv[key] = "[REDACTED]"
					} else {
						sanitizedEnv[key] = value
					}
				default:
					sanitizedEnv[key] = value
				}
			}
			sanitized[i] = sanitizedEnv
		} else {
			sanitized[i] = item
		}
	}
	return sanitized
}

// isSensitiveEnvVar checks if an environment variable map contains sensitive data
func (s *Sanitizer) isSensitiveEnvVar(envMap map[string]interface{}) bool {
	if name, ok := envMap["name"].(string); ok {
		// Check if the env var name matches sensitive patterns
		for _, pattern := range SensitiveFieldPatterns {
			if pattern.MatchString(name) {
				return true
			}
		}
	}
	return false
}

// sanitizeString sanitizes a string value
func (s *Sanitizer) sanitizeString(str, fieldName string) string {
	if s.isSensitiveField(fieldName) {
		return "[REDACTED]"
	}

	// Check if the string value contains sensitive patterns
	for _, pattern := range SensitiveValuePatterns {
		if pattern.MatchString(str) {
			return "[REDACTED]"
		}
	}

	return str
}

// isSensitiveField checks if a field name indicates sensitive data
func (s *Sanitizer) isSensitiveField(fieldName string) bool {
	// Check exact matches in deny list
	if SensitiveFieldDenyList[fieldName] {
		return true
	}

	// Check regex patterns
	for _, pattern := range SensitiveFieldPatterns {
		if pattern.MatchString(fieldName) {
			return true
		}
	}

	return false
}
