// Package article handles reading and writing local Markdown article files with YAML frontmatter.
package article

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the metadata stored in the YAML header of an article file.
type Frontmatter struct {
	Title         string    `yaml:"title"`
	Date          time.Time `yaml:"date"`
	Draft         bool      `yaml:"draft"`
	Category      []string  `yaml:"category,omitempty"`
	URL           string    `yaml:"url,omitempty"`
	EditURL       string    `yaml:"editUrl,omitempty"`
	CustomURLPath string    `yaml:"customUrlPath,omitempty"`
}

// split separates the raw file content into the YAML frontmatter string and the body string.
// Returns ("", content) if no frontmatter delimiter is found.
func split(content string) (yamlPart, body string, err error) {
	const delim = "---"
	if !strings.HasPrefix(content, delim) {
		return "", content, nil
	}
	// skip the opening "---\n"
	rest := content[len(delim):]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	idx := strings.Index(rest, "\n"+delim)
	for idx >= 0 {
		after := rest[idx+1+len(delim):]
		if len(after) == 0 || after[0] == '\n' {
			break
		}
		// closing "---" must be followed by newline or EOF; keep searching
		next := strings.Index(rest[idx+1:], "\n"+delim)
		if next < 0 {
			idx = -1
			break
		}
		idx = idx + 1 + next
	}
	if idx < 0 {
		return "", content, fmt.Errorf("frontmatter: closing --- not found")
	}
	yamlPart = rest[:idx]
	body = rest[idx+1+len(delim):]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return yamlPart, body, nil
}

// parseFrontmatter parses the YAML string into a Frontmatter struct.
func parseFrontmatter(yamlStr string) (*Frontmatter, error) {
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter YAML: %w", err)
	}
	return &fm, nil
}

// RenderFrontmatter encodes a Frontmatter back to a YAML string with --- delimiters.
func RenderFrontmatter(fm *Frontmatter) (string, error) {
	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("render frontmatter: %w", err)
	}
	return "---\n" + string(data) + "---\n", nil
}
