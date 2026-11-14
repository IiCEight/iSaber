package publishing

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdownUpdatedAt(t *testing.T) {
	for _, value := range []string{"2024-06-15T18:30:00+08:00", "2024-06-15T10:30:00Z", "\"2024-06-15T18:30:00+08:00\""} {
		parsed, err := parseMarkdown([]byte("---\nupdated_at: "+value+"\n---\n# Old article"), "old.md")
		if err != nil || len(parsed.Issues) != 0 || parsed.UpdatedAt == nil || parsed.UpdatedAt.Format(time.RFC3339) != "2024-06-15T10:30:00Z" {
			t.Fatalf("updated_at %q: %+v, %v", value, parsed, err)
		}
	}
	for _, value := range []string{"2024-02-30T18:30:00+08:00", "2024-06-15", "2024-06-15T18:30:00", "0000-01-01T00:00:00Z", "0001-01-01T00:00:00+08:00", "9999-12-31T23:59:59-08:00", "2024-06-15T10:30:00+24:00", "yesterday", "\"\""} {
		parsed, err := parseMarkdown([]byte("---\nupdated_at: "+value+"\n---\n# Old article"), "old.md")
		if err != nil || parsed.UpdatedAt != nil || len(parsed.Issues) != 1 || parsed.Issues[0].Code != "invalid_updated_at" || !blocking(parsed.Issues) {
			t.Fatalf("invalid updated_at %q: %+v, %v", value, parsed, err)
		}
	}
	for _, source := range []string{"# No timestamp", "---\nupdated_at: null\n---\n# Automatic timestamp"} {
		parsed, err := parseMarkdown([]byte(source), "old.md")
		if err != nil || parsed.UpdatedAt != nil || len(parsed.Issues) != 0 {
			t.Fatalf("optional updated_at: %+v, %v", parsed, err)
		}
	}
}

func TestMarkdownMetadataAndValidation(t *testing.T) {
	source := "\ufeff---\r\ntitle: 中文 journal\r\nwritten_at: 2026-11-07\r\nlanguage: zh\r\nvisibility: public\r\n---\r\n# Body\r\n"
	parsed, err := parseMarkdown([]byte(source), "note.md")
	if err != nil || *parsed.Title != "中文 journal" || *parsed.Language != "zh" || *parsed.WrittenAt != "2026-11-07" || parsed.Markdown != "# Body\n" {
		t.Fatalf("metadata: %+v, %v", parsed, err)
	}
	if len(parsed.Issues) != 1 || parsed.Issues[0].Code != "front_matter_ignored" {
		t.Fatal("publication control was not ignored")
	}
	for _, input := range []string{"---\n---\n# Empty metadata", "# Hello `Go`", "---\na: -----\n---\n# Heading"} {
		parsed, err := parseMarkdown([]byte(input), "fallback.md")
		if err != nil || parsed.Title == nil || strings.Contains(parsed.Markdown, "a: ----") {
			t.Fatalf("heading/front matter: %+v, %v", parsed, err)
		}
	}
	parsed, _ = parseMarkdown([]byte("# Hello `Go`"), "fallback.md")
	if *parsed.Title != "Hello Go" {
		t.Fatalf("inline heading: %s", *parsed.Title)
	}
	for _, input := range [][]byte{{0xff}, {'a', 0}, []byte("---\ntitle: &t hey\nsummary: *t\n---\nbody"), []byte("---\ntitle: a\ntitle: b\n---\nbody"), []byte(strings.Repeat("x", MaxMarkdown+1))} {
		if _, err := parseMarkdown(input, "bad.md"); err == nil {
			t.Fatal("invalid import accepted")
		}
	}
	parsed, err = parseMarkdown([]byte("---\nwritten_at: 0000-01-01\nlanguage: invalid\n---\n# Title"), "bad.md")
	if err != nil || parsed.WrittenAt != nil || parsed.Language != nil || !blocking(parsed.Issues) {
		t.Fatal("invalid metadata did not become fixable issues")
	}
}

func TestImageIssuesUseMarkdownSyntax(t *testing.T) {
	title, language, date, slug := "Images", "en", "2026-11-07", "images"
	revision := &Revision{Title: &title, Language: &language, WrittenAt: &date, Markdown: "![local][img]\n\n[img]: ./private.png\n\n```md\n![example](fake.png)\n```\n\n![remote](https://example.com/image.png)"}
	issues := revisionIssues(revision, &slug)
	if len(issues) != 2 || issues[0].Code != "missing_local_image" || issues[1].Code != "external_image" {
		t.Fatalf("image issues: %+v", issues)
	}
	if !blocking(issues) {
		t.Fatal("unavailable local image allowed publishing")
	}
	if got := excerpt("# Hello **中文**\n\nUse `Go` &amp; math."); got != "Hello 中文 Use Go & math." {
		t.Fatalf("excerpt: %q", got)
	}
}
