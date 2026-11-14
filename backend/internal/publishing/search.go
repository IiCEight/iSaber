package publishing

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
)

type searchTerm struct {
	literal string
	word    *regexp.Regexp
}

var englishTerm = regexp.MustCompile(`^[a-z0-9_]+$`)

func termsFor(query string) []searchTerm {
	terms := []searchTerm{}
	seen := map[string]bool{}
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if seen[word] {
			continue
		}
		seen[word] = true
		term := searchTerm{literal: word}
		if englishTerm.MatchString(word) {
			term.word = regexp.MustCompile(`(^|[^a-z0-9_])` + regexp.QuoteMeta(word) + `($|[^a-z0-9_])`)
		}
		terms = append(terms, term)
	}
	return terms
}
func (t searchTerm) index(value string) int {
	if t.word != nil {
		if found := t.word.FindStringIndex(value); found != nil {
			return found[0]
		}
		return -1
	}
	return strings.Index(value, t.literal)
}

// Search visible Markdown text, including code, rather than destinations or
// executable/raw HTML blocks. Inline formatting does not split visible words.
func searchableText(markdown string) string {
	source := []byte(markdown)
	doc := parser.New(parser.WithExtensions(extension.NewGFMParser())).Parse(source)
	var out strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch value := n.(type) {
			case *ast.Text:
				out.WriteString(value.Value.Value(source))
				if value.SoftLineBreak() || value.HardLineBreak() {
					out.WriteByte(' ')
				}
			case *ast.CodeSpan:
				out.WriteString(value.Value.Value(source))
			case *ast.CodeBlock:
				out.WriteString(value.Value.Str(source))
			case *ast.AutoLink:
				out.WriteString(value.Label.Value(source))
			case *ast.Image:
				out.WriteByte(' ')
			}
		} else {
			if _, block := n.(ast.BlockNode); block {
				out.WriteByte(' ')
			}
			if _, image := n.(*ast.Image); image {
				out.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(out.String()), " ")
}

func searchRank(ctx context.Context, terms []searchTerm, title, metadata, body string) (int, error) {
	rank := 0
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		current := -1
		switch {
		case term.index(title) >= 0:
			current = 0
		case term.index(metadata) >= 0:
			current = 1
		case term.index(body) >= 0:
			current = 2
		}
		if current < 0 {
			return -1, nil
		}
		if current > rank {
			rank = current
		}
	}
	return rank, nil
}
func searchSnippet(text string, terms []searchTerm) string {
	normalized := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(normalized)
	position := -1
	for _, term := range terms {
		at := term.index(lower)
		if at >= 0 && (position < 0 || at < position) {
			position = at
		}
	}
	// Case folding can change UTF-8 byte lengths; translate by rune count.
	center := 0
	if position >= 0 {
		center = utf8.RuneCountInString(lower[:position])
	}
	runes := []rune(normalized)
	start := center - 60
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := start + 240
	if end > len(runes) {
		end = len(runes)
	}
	result := string(runes[start:end])
	if start > 0 {
		result = "…" + result
	}
	if end < len(runes) {
		result += "…"
	}
	return result
}

type searchResult struct {
	article publicArticleSummary
	rank    int
}

// Negative means a sorts before b (best rank, then newest publication/ID).
func compareSearch(a, b searchResult) int {
	if a.rank != b.rank {
		if a.rank < b.rank {
			return -1
		}
		return 1
	}
	if !a.article.PublishedAt.Equal(b.article.PublishedAt) {
		if a.article.PublishedAt.After(b.article.PublishedAt) {
			return -1
		}
		return 1
	}
	return -strings.Compare(a.article.ID, b.article.ID)
}

type searchHeap []searchResult

func (h searchHeap) Len() int           { return len(h) }
func (h searchHeap) Less(i, j int) bool { return compareSearch(h[i], h[j]) > 0 }
func (h searchHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *searchHeap) Push(value any)    { *h = append(*h, value.(searchResult)) }
func (h *searchHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}

type searchCursor struct {
	Kind   string    `json:"kind"`
	Filter string    `json:"filter"`
	Rank   int       `json:"rank"`
	Time   time.Time `json:"time"`
	ID     string    `json:"id"`
}

func (s *Service) searchPublic(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	for key, items := range values {
		if (key != "q" && key != "language" && key != "category" && key != "tag" && key != "limit" && key != "cursor" && key != "page" && key != "sort") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_search_parameters"))
			return
		}
	}
	query := strings.Join(strings.Fields(values.Get("q")), " ")
	if query == "" {
		reply(w, nil, fault(400, "search_query_required"))
		return
	}
	if !utf8.ValidString(values.Get("q")) || utf8.RuneCountInString(values.Get("q")) > 200 {
		reply(w, nil, fault(400, "invalid_search_query"))
		return
	}
	values.Set("q", strings.ToLower(query))
	language, category, tag := values.Get("language"), values.Get("category"), values.Get("tag")
	if (values.Has("language") && language != "en" && language != "zh") || (values.Has("category") && (len(category) > 160 || !slugPattern.MatchString(category))) || (values.Has("tag") && (len(tag) > 160 || !slugPattern.MatchString(tag))) {
		reply(w, nil, fault(400, "invalid_filters"))
		return
	}
	options, err := parseNumberedOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	limit := 20
	if values.Has("limit") {
		number, err := strconv.Atoi(values.Get("limit"))
		if err != nil || number < 1 || number > 100 {
			reply(w, nil, fault(400, "invalid_limit"))
			return
		}
		limit = number
	}
	filterValues := make(url.Values)
	for key, items := range values {
		filterValues[key] = items
	}
	filterValues.Del("cursor")
	filterValues.Del("limit")
	filterValues.Del("page")
	hash := sha256.Sum256([]byte(filterValues.Encode()))
	fingerprint := hex.EncodeToString(hash[:])
	var after *searchResult
	if values.Has("cursor") {
		raw := values.Get("cursor")
		if len(raw) > 4096 {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		var decoded searchCursor
		if json.Unmarshal(data, &decoded) != nil || decoded.Kind != "search-v1" || decoded.Filter != fingerprint || decoded.Rank < 0 || decoded.Rank > 2 || decoded.Time.IsZero() {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		canonical, err := canonicalID(decoded.ID)
		if err != nil {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		after = &searchResult{article: publicArticleSummary{ID: canonical, PublishedAt: decoded.Time}, rank: decoded.Rank}
	}
	terms := termsFor(query)
	select {
	case s.searchSlots <- struct{}{}:
		defer func() { <-s.searchSlots }()
	default:
		w.Header().Set("Retry-After", "1")
		reply(w, nil, fault(503, "search_busy"))
		return
	}
	if options != nil {
		s.searchPublicNumbered(w, r, values, terms, *options)
		return
	}
	// One SQL snapshot reads only current public revisions. Stream candidates and
	// retain at most limit+1 summaries; never retain an entire Markdown corpus.
	rows, err := s.db.QueryContext(r.Context(), publicSearchSQL, language, category, tag)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	best := &searchHeap{}
	heap.Init(best)
	for rows.Next() {
		if err := r.Context().Err(); err != nil {
			reply(w, nil, err)
			return
		}
		result, text, err := scanSearchResult(r.Context(), rows, terms)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if result.rank < 0 {
			continue
		}
		article, rank := result.article, result.rank
		if after != nil && compareSearch(result, *after) <= 0 {
			continue
		}
		if best.Len() >= limit+1 && compareSearch(result, (*best)[0]) >= 0 {
			continue
		}
		if rank == 2 || article.Summary == "" {
			result.article.Summary = searchSnippet(text, terms)
		} else {
			result.article.Summary = searchSnippet(article.Summary, terms)
		}
		if best.Len() >= limit+1 {
			heap.Pop(best)
		}
		heap.Push(best, result)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	sort.Slice(*best, func(i, j int) bool { return compareSearch((*best)[i], (*best)[j]) < 0 })
	more := best.Len() > limit
	if more {
		*best = (*best)[:limit]
	}
	items := []publicArticleSummary{}
	for _, result := range *best {
		items = append(items, result.article)
	}
	var next *string
	if more && len(items) > 0 {
		last := (*best)[len(items)-1]
		encoded, _ := json.Marshal(searchCursor{Kind: "search-v1", Filter: fingerprint, Rank: last.rank, Time: last.article.PublishedAt, ID: last.article.ID})
		cursor := base64.RawURLEncoding.EncodeToString(encoded)
		next = &cursor
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}
