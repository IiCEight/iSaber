package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

func scanSearchResult(ctx context.Context, rows *sql.Rows, terms []searchTerm) (searchResult, string, error) {
	var article publicArticleSummary
	var categoryJSON, tagsJSON []byte
	var markdown string
	if err := rows.Scan(&article.ID, &article.Slug, &article.Title, &article.Summary, &article.Language, &article.WrittenAt, &article.PublishedAt, &article.UpdatedAt, &categoryJSON, &tagsJSON, &markdown); err != nil {
		return searchResult{}, "", err
	}
	if err := json.Unmarshal(categoryJSON, &article.Category); err != nil {
		return searchResult{}, "", err
	}
	if err := json.Unmarshal(tagsJSON, &article.Tags); err != nil {
		return searchResult{}, "", err
	}
	text := searchableText(markdown)
	metadata := article.Summary
	for _, label := range article.Tags {
		metadata += "\n" + label.Name
	}
	rank, err := searchRank(ctx, terms, strings.ToLower(article.Title), strings.ToLower(metadata), strings.ToLower(text))
	article.PublishedAt = article.PublishedAt.UTC()
	article.UpdatedAt = article.UpdatedAt.UTC()
	return searchResult{article: article, rank: rank}, text, err
}

func (s *Service) searchPublicNumbered(w http.ResponseWriter, r *http.Request, values url.Values, terms []searchTerm, options numberedOptions) {
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	args := []any{values.Get("language"), values.Get("category"), values.Get("tag")}
	query := publicSearchSQL + ` ORDER BY ` + publicArticleOrder(options.Sort)
	// Count each rank band first. A second ordered scan selects only the requested
	// slice, keeping memory bounded to one page even for a very large page number.
	counts := [3]int64{}
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		result, _, err := scanSearchResult(r.Context(), rows, terms)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if result.rank >= 0 {
			band := result.rank
			if options.Sort != "relevance" {
				band = 0
			}
			counts[band]++
		}
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	rows.Close()
	page := options.result(counts[0] + counts[1] + counts[2])
	if page.Total == 0 {
		reply(w, page, nil)
		return
	}
	rows, err = tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	starts := [3]int64{0, counts[0], counts[0] + counts[1]}
	seen := [3]int64{}
	selected := []searchResult{}
	for rows.Next() {
		result, text, err := scanSearchResult(r.Context(), rows, terms)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if result.rank < 0 {
			continue
		}
		band := result.rank
		if options.Sort != "relevance" {
			band = 0
		}
		position := starts[band] + seen[band]
		seen[band]++
		if position < page.offset() || position >= page.offset()+int64(page.PageSize) {
			continue
		}
		if result.rank == 2 || result.article.Summary == "" {
			result.article.Summary = searchSnippet(text, terms)
		} else {
			result.article.Summary = searchSnippet(result.article.Summary, terms)
		}
		selected = append(selected, result)
		if len(selected) == page.PageSize {
			break
		}
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	if options.Sort == "relevance" {
		sort.Slice(selected, func(i, j int) bool { return compareSearch(selected[i], selected[j]) < 0 })
	}
	for _, result := range selected {
		page.Items = append(page.Items, result.article)
	}
	reply(w, page, nil)
}

const publicSearchSQL = `SELECT a.id,a.slug,r.title,COALESCE(r.summary,''),r.language,to_char(r.written_at,'YYYY-MM-DD'),a.published_at,a.updated_at,
 CASE WHEN c.id IS NULL THEN 'null'::jsonb ELSE jsonb_build_object('id',c.id,'name',c.name,'slug',c.slug) END,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'name',t.name,'slug',t.slug) ORDER BY t.name,t.id)
 FROM tags t JOIN revision_tags rt ON rt.tag_id=t.id WHERE rt.revision_id=r.id),'[]'::jsonb),r.markdown
 FROM articles a JOIN revisions r ON r.id=a.published_revision_id LEFT JOIN categories c ON c.id=r.category_id
 WHERE a.visibility='public' AND ($1='' OR r.language=$1) AND ($2='' OR c.slug=$2)
 AND ($3='' OR EXISTS(SELECT 1 FROM revision_tags rt JOIN tags t ON t.id=rt.tag_id WHERE rt.revision_id=r.id AND t.slug=$3))`
