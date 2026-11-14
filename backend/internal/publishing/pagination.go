package publishing

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

var pageNumberPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

type numberedOptions struct {
	Page int
	Size int
	Sort string
}

// Numbered pages are opt-in. Existing cursor clients keep their response shape.
func parseNumberedOptions(values url.Values) (*numberedOptions, error) {
	if !values.Has("page") {
		if values.Has("sort") {
			return nil, fault(400, "sort_requires_page")
		}
		return nil, nil
	}
	if len(values["page"]) != 1 || values.Has("cursor") || !pageNumberPattern.MatchString(values.Get("page")) {
		return nil, fault(400, "invalid_page")
	}
	number, err := strconv.ParseInt(values.Get("page"), 10, 32)
	if err != nil {
		return nil, fault(400, "invalid_page")
	}
	size := 20
	if values.Has("limit") {
		if len(values["limit"]) != 1 {
			return nil, fault(400, "invalid_limit")
		}
		size, err = strconv.Atoi(values.Get("limit"))
		if err != nil || size < 1 || size > 100 {
			return nil, fault(400, "invalid_limit")
		}
	}
	order := values.Get("sort")
	if !values.Has("sort") {
		order = "written"
	}
	if values.Has("sort") && len(values["sort"]) != 1 {
		return nil, fault(400, "invalid_sort")
	}
	switch order {
	case "written", "written_oldest", "updated", "updated_oldest", "newest", "oldest":
	case "relevance":
		if values.Get("q") == "" {
			return nil, fault(400, "invalid_sort")
		}
	default:
		return nil, fault(400, "invalid_sort")
	}
	return &numberedOptions{Page: int(number), Size: size, Sort: order}, nil
}

func publicArticleOrder(order string) string {
	switch order {
	case "written_oldest":
		return "r.written_at ASC NULLS LAST,a.id ASC"
	case "updated":
		return "a.updated_at DESC NULLS LAST,a.id DESC"
	case "updated_oldest":
		return "a.updated_at ASC NULLS LAST,a.id ASC"
	case "oldest":
		return "a.published_at ASC,a.id ASC"
	case "newest", "relevance":
		return "a.published_at DESC,a.id DESC"
	default:
		return "r.written_at DESC NULLS LAST,a.id DESC"
	}
}

type numberedArticlePage struct {
	Items      []publicArticleSummary `json:"items"`
	NextCursor *string                `json:"next_cursor"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	Total      int64                  `json:"total"`
	TotalPages int                    `json:"total_pages"`
}

func (options numberedOptions) result(total int64) numberedArticlePage {
	pages := max(1, int((total+int64(options.Size)-1)/int64(options.Size)))
	return numberedArticlePage{Items: []publicArticleSummary{}, Page: min(options.Page, pages), PageSize: options.Size, Total: total, TotalPages: pages}
}

func (page numberedArticlePage) offset() int64 {
	return int64(page.Page-1) * int64(page.PageSize)
}

const publicCollectionSQL = ` FROM articles a JOIN revisions r ON r.id=a.published_revision_id LEFT JOIN categories c ON c.id=r.category_id
 WHERE a.visibility='public' AND ($1='' OR r.language=$1) AND ($2='' OR c.slug=$2)
 AND ($3='' OR EXISTS(SELECT 1 FROM revision_tags rt JOIN tags t ON t.id=rt.tag_id WHERE rt.revision_id=r.id AND t.slug=$3))`

func (s *Service) listPublicNumbered(w http.ResponseWriter, r *http.Request, values url.Values, options numberedOptions) {
	for key, items := range values {
		if (key != "page" && key != "limit" && key != "language" && key != "category" && key != "tag" && key != "sort") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_page_parameters"))
			return
		}
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	args := []any{values.Get("language"), values.Get("category"), values.Get("tag")}
	var total int64
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*)`+publicCollectionSQL, args...).Scan(&total); err != nil {
		reply(w, nil, err)
		return
	}
	page := options.result(total)
	args = append(args, page.PageSize, page.offset())
	rows, err := tx.QueryContext(r.Context(), `SELECT a.slug`+publicCollectionSQL+` ORDER BY `+publicArticleOrder(options.Sort)+` LIMIT $4 OFFSET $5`, args...)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	slugs := []string{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			reply(w, nil, err)
			return
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	rows.Close()
	for _, slug := range slugs {
		article, err := readPublicArticle(r.Context(), tx, slug)
		if err != nil {
			reply(w, nil, err)
			return
		}
		page.Items = append(page.Items, article.publicArticleSummary)
	}
	reply(w, page, nil)
}
