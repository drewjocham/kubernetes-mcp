package util

import "strings"

// DecodeHTMLEntities decodes common HTML entities in text.
func DecodeHTMLEntities(input string) string {
	result := input
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	return result
}

// CleanupMarkdown cleans up markdown formatting issues, decodes HTML entities,
// removes HTML tags, and normalizes whitespace.
func CleanupMarkdown(input string) string {
	if input == "" {
		return input
	}

	// Decode HTML entities first
	result := DecodeHTMLEntities(input)

	// Replace HTML line breaks with newlines
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")

	// Fix common malformed patterns
	// Remove duplicate consecutive code block markers
	lines := strings.Split(result, "\n")
	var cleanedLines []string
	inCodeBlock := false
	prevLine := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a code block
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// Already in code block, might be duplicate opener
				// Skip if previous line was also a code block opener
				if strings.HasPrefix(strings.TrimSpace(prevLine), "```") {
					continue // Skip duplicate opener
				}
			}
			inCodeBlock = !inCodeBlock
		}

		cleanedLines = append(cleanedLines, line)
		prevLine = line
	}

	result = strings.Join(cleanedLines, "\n")

	// Remove any remaining HTML tags (simple approach)
	result = strings.ReplaceAll(result, "<strong>", "**")
	result = strings.ReplaceAll(result, "</strong>", "**")
	result = strings.ReplaceAll(result, "<b>", "**")
	result = strings.ReplaceAll(result, "</b>", "**")
	result = strings.ReplaceAll(result, "<em>", "*")
	result = strings.ReplaceAll(result, "</em>", "*")
	result = strings.ReplaceAll(result, "<i>", "*")
	result = strings.ReplaceAll(result, "</i>", "*")

	// Remove any other HTML tags (crude but works for common cases)
	for strings.Contains(result, "<") && strings.Contains(result, ">") {
		start := strings.Index(result, "<")
		end := strings.Index(result, ">")
		if start >= 0 && end > start {
			result = result[:start] + result[end+1:]
		} else {
			break
		}
	}

	// Normalize newlines (3+ newlines -> 2 newlines)
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(result)
}

// MaskAPIKey masks an API key for logging purposes.
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}
