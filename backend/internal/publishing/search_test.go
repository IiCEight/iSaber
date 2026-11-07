package publishing

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSearchWordsLiteralsAndRanking(t *testing.T) {
	for _, test := range []struct {
		query, title, metadata, body string
		rank                         int
	}{
		{"Go", "go modules", "", "", 0}, {"go", "Golang", "", "", -1},
		{"go", "Gopher", "go", "", 1}, {"go", "", "", "Go code", 2},
		{"Go modules", "Go", "", "modules", 2}, {"Go modules", "Go modules", "", "", 0},
		{"Go missing", "Go", "", "other", -1},
		{"中文", "包含中文的标题", "", "", 0}, {"中文", "", "", "连续中文文本", 2},
		{"C++", "C++ examples", "", "", 0}, {"100%", "1000", "", "", -1},
		{".*", "anything", "", "", -1}, {".*", "Literal .* pattern", "", "", 0},
		{"some_id", "some_identifier", "", "", -1}, {"some_id", "(some_id)", "", "", 0},
		{"数据库 Go", "数据库", "go", "", 1},
	} {
		rank, err := searchRank(context.Background(), termsFor(test.query), strings.ToLower(test.title), strings.ToLower(test.metadata), strings.ToLower(test.body))
		if err != nil || rank != test.rank {
			t.Fatalf("query %q: got %d, want %d, error %v", test.query, rank, test.rank, err)
		}
	}
}
func TestSearchUsesVisibleMarkdownAndReadableSnippets(t *testing.T) {
	source := "# G**o** &amp; 中文\n\nA [label](https://hidden-url.example/pathsecret) and `InlineCode`.\n\n```go\ncode_token()\n```\n\n<script>hidden_script</script>\n\n<https://visible.example>"
	text := searchableText(source)
	for _, needle := range []string{"Go & 中文", "label", "InlineCode", "code_token()", "https://visible.example"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("visible text missing %q from %q", needle, text)
		}
	}
	for _, needle := range []string{"hidden-url", "pathsecret", "hidden_script", "```", "**"} {
		if strings.Contains(text, needle) {
			t.Fatalf("non-visible source indexed: %q", needle)
		}
	}
	snippet := searchSnippet(strings.Repeat("背景 ", 300)+"needle 中文命中 "+strings.Repeat("ending ", 100), termsFor("中文命中"))
	if !utf8.ValidString(snippet) || !strings.Contains(snippet, "中文命中") || utf8.RuneCountInString(snippet) > 242 || !strings.HasPrefix(snippet, "…") || !strings.HasSuffix(snippet, "…") {
		t.Fatalf("snippet: %q", snippet)
	}
}
