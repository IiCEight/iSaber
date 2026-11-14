package publishing

import (
	"bytes"
	"errors"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"go.yaml.in/yaml/v3"
)

const MaxMarkdown = 5 * 1024 * 1024

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var frontMatterEnd = regexp.MustCompile(`(?m)^---[ \t]*(?:\n|$)`)
var timestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$`)

var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

func validSlug(value string) bool {
	return len(value) <= 160 && slugPattern.MatchString(value) && value != "search"
}

type imported struct {
	Markdown                            string
	Fields                              map[string]bool
	Title, Summary, Language, WrittenAt *string
	UpdatedAt                           *time.Time
	Slug                                *string
	Category                            *string
	Tags                                []string
	Issues                              []Issue
}

func parseMarkdown(data []byte, filename string) (imported, error) {
	result := imported{Fields: map[string]bool{}, Issues: []Issue{}}
	if len(data) > MaxMarkdown || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return result, errors.New("invalid_markdown")
	}
	result.Markdown = strings.ReplaceAll(strings.TrimPrefix(string(data), "\ufeff"), "\r\n", "\n")
	if strings.HasPrefix(result.Markdown, "---\n") {
		if match := frontMatterEnd.FindStringIndex(result.Markdown[4:]); match != nil {
			end := match[0] + 4
			closing := match[1] + 4
			if end > 64*1024 {
				return result, errors.New("front_matter_too_large")
			}
			var node yaml.Node
			metadata := result.Markdown[4:end]
			if strings.TrimSpace(metadata) == "" {
				metadata = "{}"
			}
			if err := yaml.Unmarshal([]byte(metadata), &node); err != nil {
				return result, errors.New("invalid_front_matter")
			}
			if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode || hasAlias(node.Content[0]) {
				return result, errors.New("invalid_front_matter")
			}
			var meta struct {
				Title     *string  `yaml:"title"`
				Summary   *string  `yaml:"summary"`
				Slug      *string  `yaml:"slug"`
				Language  *string  `yaml:"language"`
				WrittenAt *string  `yaml:"written_at"`
				UpdatedAt *string  `yaml:"updated_at"`
				Category  *string  `yaml:"category"`
				Tags      []string `yaml:"tags"`
			}
			if err := node.Decode(&meta); err != nil {
				return result, errors.New("invalid_front_matter")
			}
			known := map[string]bool{"title": true, "summary": true, "slug": true, "language": true, "written_at": true, "updated_at": true, "category": true, "tags": true}
			for i := 0; i < len(node.Content[0].Content); i += 2 {
				key := node.Content[0].Content[i].Value
				result.Fields[key] = true
				if !known[key] {
					result.Issues = append(result.Issues, issue("warning", "front_matter_ignored", key, "This front-matter field is not applied. Visibility and publication are controlled in the dashboard."))
				}
			}
			result.Title, result.Summary, result.Slug = meta.Title, meta.Summary, meta.Slug
			result.Language, result.WrittenAt, result.Category, result.Tags = meta.Language, meta.WrittenAt, meta.Category, meta.Tags
			if meta.UpdatedAt != nil {
				value, err := parseUpdatedAt(*meta.UpdatedAt)
				if err != nil {
					result.Issues = append(result.Issues, issue("blocking", "invalid_updated_at", "updated_at", "Enter a valid last updated time with a timezone, or clear it to use publication time."))
				} else {
					result.UpdatedAt = &value
				}
			}
			result.Markdown = strings.TrimPrefix(result.Markdown[closing:], "\n")
		}
	}
	if !result.Fields["title"] {
		title := strings.TrimSuffix(path.Base(filename), path.Ext(filename))
		source := []byte(result.Markdown)
		document := parser.New().Parse(source)
		_ = ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				if heading, ok := n.(*ast.Heading); ok && heading.Level == 1 {
					title = plainNode(n, source)
					return ast.WalkStop, nil
				}
			}
			return ast.WalkContinue, nil
		})
		result.Title = &title
	}
	for _, field := range []struct {
		name  string
		value **string
		max   int
	}{{"title", &result.Title, 300}, {"summary", &result.Summary, 2000}} {
		if *field.value != nil && utf8.RuneCountInString(**field.value) > field.max {
			result.Issues = append(result.Issues, issue("blocking", "metadata_too_long", field.name, "Shorten this field before publishing."))
			*field.value = nil
		}
	}
	if result.Language != nil && *result.Language != "en" && *result.Language != "zh" {
		result.Language = nil
		result.Issues = append(result.Issues, issue("blocking", "invalid_language", "language", "Choose English or Chinese."))
	}
	if result.WrittenAt != nil {
		if !validDate(*result.WrittenAt) {
			result.WrittenAt = nil
			result.Issues = append(result.Issues, issue("blocking", "invalid_written_at", "written_at", "Choose a valid writing date."))
		}
	}
	return result, nil
}

func hasAlias(node *yaml.Node) bool {
	if node.Kind == yaml.AliasNode {
		return true
	}
	for _, child := range node.Content {
		if hasAlias(child) {
			return true
		}
	}
	return false
}

func revisionIssues(revision *Revision, slug *string) []Issue {
	issues := append([]Issue{}, revision.SourceIssues...)
	if revision.Title == nil || strings.TrimSpace(*revision.Title) == "" {
		issues = append(issues, issue("blocking", "title_required", "title", "Enter an article title."))
	}
	if slug == nil || !validSlug(*slug) {
		issues = append(issues, issue("blocking", "slug_required", "slug", "Choose a unique URL using lowercase letters, numbers, and hyphens. The name search is reserved."))
	}
	if revision.Language == nil {
		issues = append(issues, issue("blocking", "language_required", "language", "Choose English or Chinese."))
	}
	if revision.WrittenAt == nil {
		issues = append(issues, issue("blocking", "written_at_required", "written_at", "Choose the writing date before publishing."))
	}
	source := []byte(revision.Markdown)
	document := parser.New(parser.WithExtensions(extension.NewGFMParser())).Parse(source)
	seen := map[string]bool{}
	_ = ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if image, ok := n.(*ast.Image); ok {
			destination := image.Destination.Value(source)
			if !seen[destination] {
				seen[destination] = true
				parsed, err := url.Parse(destination)
				if hasManagedImage(revision, destination) {
					// Revision-owned managed images are validated by import and publication.
				} else if err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil {
					issues = append(issues, issue("warning", "external_image", destination, "This image loads from another host; the original URL is preserved."))
				} else {
					issues = append(issues, issue("blocking", "missing_local_image", destination, "This image is unavailable. Upload a ZIP containing the Markdown file and its relative image files."))
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return issues
}

func excerpt(markdown string) string {
	source := []byte(markdown)
	document := parser.New().Parse(source)
	var out strings.Builder
	_ = ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if value, ok := n.(*ast.Text); ok {
				out.WriteString(value.Value.Value(source))
				if value.SoftLineBreak() || value.HardLineBreak() {
					out.WriteByte(' ')
				}
			}
			if value, ok := n.(*ast.CodeSpan); ok {
				out.WriteString(value.Value.Value(source))
			}
		} else if _, ok := n.(ast.BlockNode); ok {
			out.WriteByte(' ')
		}
		return ast.WalkContinue, nil
	})
	runes := []rune(html.UnescapeString(strings.Join(strings.Fields(out.String()), " ")))
	if len(runes) > 240 {
		return string(runes[:240]) + "…"
	}
	return string(runes)
}

func plainNode(node ast.Node, source []byte) string {
	var out strings.Builder
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if text, ok := n.(*ast.Text); ok {
				out.WriteString(text.Value.Value(source))
			}
			if value, ok := n.(*ast.CodeSpan); ok {
				out.WriteString(value.Value.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
	return out.String()
}

func validDate(value string) bool {
	date, err := time.Parse("2006-01-02", value)
	return err == nil && date.Year() >= 1
}

func parseUpdatedAt(value string) (time.Time, error) {
	date, err := time.Parse(time.RFC3339Nano, value)
	date = date.UTC()
	if err != nil || !timestampPattern.MatchString(value) || date.Year() < 1 || date.Year() > 9999 {
		return time.Time{}, errors.New("invalid_updated_at")
	}
	return date, nil
}
