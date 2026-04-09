package core

import (
	"regexp"
	"strings"
)

// ExtractMetadataValue extracts a value from "- Label: value" format in markdown.
func ExtractMetadataValue(text, label string) string {
	pattern := regexp.MustCompile(`(?m)^- ` + regexp.QuoteMeta(label) + `: (.+)$`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// ExtractSection extracts the content under a ## heading.
func ExtractSection(text, heading string) string {
	escaped := regexp.QuoteMeta(heading)
	pattern := regexp.MustCompile(`(?ms)^## ` + escaped + `\n(.*?)(?:^## |\z)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// ExtractChecklistItems returns all checklist items under a heading.
func ExtractChecklistItems(text, heading string) []string {
	section := ExtractSection(text, heading)
	var items []string
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			items = append(items, trimmed)
		}
	}
	return items
}

// IsPlaceholderSection returns true if a section is empty or just "pending".
func IsPlaceholderSection(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	return normalized == "" || normalized == "pending" || normalized == "- pending"
}

// ParseMarkdownTableRows parses rows from a markdown table following a header line.
func ParseMarkdownTableRows(markdown, headerPrefix string) [][]string {
	lines := strings.Split(markdown, "\n")
	var rows [][]string
	inTable := false

	for _, line := range lines {
		if !inTable && strings.HasPrefix(line, headerPrefix) {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if strings.Contains(line, "|---|") {
			continue
		}

		parts := splitTableRow(line)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		rows = append(rows, parts)
	}
	return rows
}

// splitTableRow splits a markdown table row into cell values.
func splitTableRow(line string) []string {
	// Remove leading/trailing pipes and split
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}

	parts := strings.Split(trimmed, "|")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.TrimSpace(p)
	}
	return result
}
