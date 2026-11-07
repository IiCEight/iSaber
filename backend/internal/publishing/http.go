package publishing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/google/uuid"
)

func (s *Service) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/articles", s.listAdmin)
	mux.HandleFunc("GET /api/v1/admin/articles/{article_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "article_id")
		if !ok {
			return
		}
		a, err := loadArticle(r.Context(), s.db, id)
		reply(w, a, err)
	})
	mux.HandleFunc("GET /api/v1/admin/articles/{article_id}/preview", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "article_id")
		if !ok {
			return
		}
		a, err := loadArticle(r.Context(), s.db, id)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if a.Draft == nil {
			reply(w, nil, fault(404, "not_found"))
			return
		}
		reply(w, map[string]any{"article_id": id, "article_version": a.Version, "revision": a.Draft, "publishable": a.Publishable, "issues": a.Issues}, nil)
	})
	mux.HandleFunc("PATCH /api/v1/admin/articles/{article_id}/draft", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "article_id")
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if len(fields) < 2 {
			reply(w, nil, fault(422, "metadata_required"))
			return
		}
		a, err := s.PatchDraft(r.Context(), id, fields)
		reply(w, a, err)
	})
	mux.HandleFunc("PATCH /api/v1/admin/articles/{article_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "article_id")
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if len(fields) < 2 {
			reply(w, nil, fault(422, "controls_required"))
			return
		}
		a, err := s.Controls(r.Context(), id, fields)
		reply(w, a, err)
	})
	for _, operation := range []string{"publish", "unpublish"} {
		mux.HandleFunc("POST /api/v1/admin/articles/{article_id}/"+operation, func(w http.ResponseWriter, r *http.Request) {
			id, ok := pathID(w, r, "article_id")
			if !ok {
				return
			}
			key, ok := requestKey(w, r)
			if !ok {
				return
			}
			fields, err := readFields(w, r)
			if err != nil {
				reply(w, nil, err)
				return
			}
			owner, _ := auth.OwnerFromContext(r.Context())
			result, err := s.Publish(r.Context(), owner.ID, id, key, fields, operation == "unpublish")
			sendResult(w, result, err)
		})
	}
	mux.HandleFunc("POST /api/v1/admin/imports", s.importFile)
	mux.HandleFunc("GET /api/v1/admin/imports/{import_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "import_id")
		if !ok {
			return
		}
		job, err := s.GetJob(r.Context(), id)
		reply(w, job, err)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if _, ok := auth.OwnerFromContext(r.Context()); !ok {
			reply(w, nil, fault(401, "unauthorized"))
			return
		}
		serveRoutes(mux, w, r)
	})
}

func (s *Service) PublicHandler(assetHandlers ...http.Handler) http.Handler {
	mux := http.NewServeMux()
	assetHandler := s.AssetHandler()
	if len(assetHandlers) > 0 {
		assetHandler = assetHandlers[0]
	}
	mux.Handle("GET /api/v1/articles/{article_id}/assets/{asset_id}", assetHandler)
	mux.HandleFunc("GET /api/v1/articles", s.listPublic)
	mux.HandleFunc("GET /api/v1/articles/{slug}", func(w http.ResponseWriter, r *http.Request) {
		value, err := s.publicArticle(r.Context(), r.PathValue("slug"))
		reply(w, value, err)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		serveRoutes(mux, w, r.WithContext(ctx))
	})
}

// Keep routing errors in the same problem format as application errors.
func serveRoutes(mux *http.ServeMux, w http.ResponseWriter, r *http.Request) {
	if _, pattern := mux.Handler(r); pattern != "" {
		mux.ServeHTTP(w, r)
		return
	}
	allowed := []string{}
	for _, method := range []string{"GET", "HEAD", "POST", "PATCH"} {
		probe := r.Clone(r.Context())
		probe.Method = method
		if _, pattern := mux.Handler(probe); pattern != "" {
			allowed = append(allowed, method)
		}
	}
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		reply(w, nil, fault(405, "method_not_allowed"))
		return
	}
	reply(w, nil, fault(404, "not_found"))
}

func (s *Service) importFile(w http.ResponseWriter, r *http.Request) {
	key, ok := requestKey(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxArchive+128*1024)
	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			reply(w, nil, fault(413, "upload_too_large"))
		} else {
			reply(w, nil, fault(400, "invalid_multipart_upload"))
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		reply(w, nil, fault(422, "one_markdown_file_required"))
		return
	}
	for field, values := range r.MultipartForm.Value {
		if (field != "article_id" && field != "expected_version") || len(values) != 1 {
			reply(w, nil, fault(422, "invalid_upload_fields"))
			return
		}
	}
	header := r.MultipartForm.File["file"][0]
	filename := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if !utf8.ValidString(filename) || len(filename) > 255 || (strings.ToLower(path.Ext(filename)) != ".md" && strings.ToLower(path.Ext(filename)) != ".zip") {
		reply(w, nil, fault(415, "markdown_or_zip_required"))
		return
	}
	maximum := int64(MaxMarkdown)
	if strings.EqualFold(path.Ext(filename), ".zip") {
		maximum = MaxArchive
	}
	if header.Size > maximum {
		reply(w, nil, fault(413, "upload_too_large"))
		return
	}
	var article *string
	var version *int64
	if value := r.FormValue("article_id"); value != "" {
		parsed, err := canonicalID(value)
		if err != nil {
			reply(w, nil, fault(422, "invalid_article_id"))
			return
		}
		article = &parsed
		number, err := strconv.ParseInt(r.FormValue("expected_version"), 10, 64)
		if err != nil || number < 1 {
			reply(w, nil, fault(422, "expected_version_required"))
			return
		}
		version = &number
	} else if r.FormValue("expected_version") != "" {
		reply(w, nil, fault(422, "article_id_required"))
		return
	}
	file, err := header.Open()
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		reply(w, nil, err)
		return
	}
	if int64(len(data)) > maximum {
		reply(w, nil, fault(413, "upload_too_large"))
		return
	}
	owner, _ := auth.OwnerFromContext(r.Context())
	result, err := s.AcceptImport(r.Context(), owner.ID, key, filename, data, article, version)
	if err == nil {
		var job Job
		if json.Unmarshal(result.Body, &job) == nil {
			w.Header().Set("Location", "/api/v1/admin/imports/"+job.ID)
		}
	}
	sendResult(w, result, err)
}

func canonicalID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || !strings.EqualFold(id.String(), value) {
		return "", errors.New("invalid_id")
	}
	return id.String(), nil
}
func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id, err := canonicalID(r.PathValue(name))
	if err != nil {
		reply(w, nil, fault(400, "invalid_id"))
		return "", false
	}
	return id, true
}
func requestKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key, err := canonicalID(r.Header.Get("Idempotency-Key"))
	if err != nil {
		reply(w, nil, fault(400, "idempotency_key_required"))
		return "", false
	}
	return key, true
}
func readFields(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, error) {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		return nil, fault(415, "unsupported_media_type")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	decoder := json.NewDecoder(r.Body)
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, fault(400, "invalid_json")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fault(400, "invalid_json")
	}
	return fields, nil
}
func reply(w http.ResponseWriter, value any, err error) {
	if err != nil {
		status, code := 503, "storage_unavailable"
		var known *Fault
		if errors.As(err, &known) {
			status, code = known.Status, known.Code
		}
		if code == "request_in_progress" {
			w.Header().Set("Retry-After", "1")
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code, "request_id": w.Header().Get("X-Request-ID")})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func sendResult(w http.ResponseWriter, result response, err error) {
	if err != nil {
		reply(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Body)
}

type cursor struct {
	Time   time.Time `json:"time"`
	ID     string    `json:"id"`
	Filter string    `json:"filter"`
}

func pageOptions(values url.Values) (int, *cursor, string, error) {
	limit := 20
	if raw, ok := values["limit"]; ok {
		number, err := strconv.Atoi(raw[0])
		if err != nil || number < 1 || number > 100 {
			return 0, nil, "", fault(400, "invalid_limit")
		}
		limit = number
	}
	copyValues := url.Values{}
	for key, items := range values {
		if key != "cursor" && key != "limit" {
			copyValues[key] = items
		}
	}
	hash := sha256.Sum256([]byte(copyValues.Encode()))
	fingerprint := hex.EncodeToString(hash[:])
	var after *cursor
	if raw, ok := values["cursor"]; ok {
		if len(raw[0]) > 4096 {
			return 0, nil, "", fault(400, "invalid_cursor")
		}
		data, err := base64.RawURLEncoding.DecodeString(raw[0])
		if err != nil {
			return 0, nil, "", fault(400, "invalid_cursor")
		}
		after = &cursor{}
		if json.Unmarshal(data, after) != nil || after.Filter != fingerprint || after.Time.IsZero() {
			return 0, nil, "", fault(400, "invalid_cursor")
		}
		if _, err := canonicalID(after.ID); err != nil {
			return 0, nil, "", fault(400, "invalid_cursor")
		}
	}
	return limit, after, fingerprint, nil
}
func encodeCursor(at time.Time, id, filter string) *string {
	data, _ := json.Marshal(cursor{at, id, filter})
	encoded := base64.RawURLEncoding.EncodeToString(data)
	return &encoded
}

func (s *Service) listAdmin(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	limit, after, filter, err := pageOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	status, visibility, query := values.Get("status"), values.Get("visibility"), values.Get("q")
	if (status != "" && status != "draft" && status != "published") || (visibility != "" && visibility != "public" && visibility != "private") || utf8.RuneCountInString(query) > 200 {
		reply(w, nil, fault(400, "invalid_filters"))
		return
	}
	var at *time.Time
	var id *string
	if after != nil {
		at = &after.Time
		id = &after.ID
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.id FROM articles a LEFT JOIN revisions d ON d.id=a.draft_revision_id
        LEFT JOIN revisions p ON p.id=a.published_revision_id WHERE ($1='' OR (CASE WHEN a.published_revision_id IS NULL THEN 'draft' ELSE 'published' END)=$1)
        AND ($2='' OR a.visibility=$2) AND ($3='' OR strpos(lower(COALESCE(d.title,p.title,'')),lower($3))>0)
        AND ($4::timestamptz IS NULL OR (a.modified_at,a.id)<($4,$5::uuid)) ORDER BY a.modified_at DESC,a.id DESC LIMIT $6`, status, visibility, query, at, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			reply(w, nil, err)
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		reply(w, nil, err)
		return
	}
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	items := []ArticleSummary{}
	for _, id := range ids {
		a, err := loadArticle(r.Context(), s.db, id)
		if err != nil {
			reply(w, nil, err)
			return
		}
		items = append(items, a.ArticleSummary)
	}
	var next *string
	if more && len(items) > 0 {
		last := items[len(items)-1]
		next = encodeCursor(last.ModifiedAt, last.ID, filter)
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}

type publicArticle struct {
	ID              string    `json:"id"`
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Language        string    `json:"language"`
	WrittenAt       string    `json:"written_at"`
	PublishedAt     time.Time `json:"published_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Category        *Label    `json:"category"`
	Tags            []Label   `json:"tags"`
	Markdown        string    `json:"markdown"`
	CommentsEnabled bool      `json:"comments_enabled"`
}

func (s *Service) publicArticle(ctx context.Context, slug string) (publicArticle, error) {
	var value publicArticle
	var revisionID string
	err := s.db.QueryRowContext(ctx, `SELECT id,slug,published_revision_id,published_at,updated_at,comments_enabled FROM articles
        WHERE slug=$1 AND visibility='public' AND published_revision_id IS NOT NULL`, slug).Scan(&value.ID, &value.Slug, &revisionID, &value.PublishedAt, &value.UpdatedAt, &value.CommentsEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return value, fault(404, "not_found")
	}
	if err != nil {
		return value, err
	}
	revision, err := loadRevision(ctx, s.db, revisionID)
	if err != nil {
		return value, err
	}
	value.Title = titleOrEmpty(revision.Title)
	value.Language = titleOrEmpty(revision.Language)
	value.WrittenAt = titleOrEmpty(revision.WrittenAt)
	value.Summary = titleOrEmpty(revision.Summary)
	if value.Summary == "" {
		value.Summary = excerpt(revision.Markdown)
	}
	value.Category, value.Tags, value.Markdown = revision.Category, revision.Tags, revision.Markdown
	value.PublishedAt = value.PublishedAt.UTC()
	value.UpdatedAt = value.UpdatedAt.UTC()
	return value, nil
}
func (s *Service) listPublic(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	limit, after, filter, err := pageOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	language, category, tag := values.Get("language"), values.Get("category"), values.Get("tag")
	if (language != "" && language != "en" && language != "zh") || len(category) > 160 || len(tag) > 160 {
		reply(w, nil, fault(400, "invalid_filters"))
		return
	}
	var at *time.Time
	var id *string
	if after != nil {
		at = &after.Time
		id = &after.ID
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.slug FROM articles a JOIN revisions r ON r.id=a.published_revision_id LEFT JOIN categories c ON c.id=r.category_id
        WHERE a.visibility='public' AND ($1='' OR r.language=$1) AND ($2='' OR c.slug=$2)
        AND ($3='' OR EXISTS(SELECT 1 FROM revision_tags rt JOIN tags t ON t.id=rt.tag_id WHERE rt.revision_id=r.id AND t.slug=$3))
        AND ($4::timestamptz IS NULL OR (a.published_at,a.id)<($4,$5::uuid)) ORDER BY a.published_at DESC,a.id DESC LIMIT $6`, language, category, tag, at, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	slugs := []string{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			rows.Close()
			reply(w, nil, err)
			return
		}
		slugs = append(slugs, slug)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		reply(w, nil, err)
		return
	}
	more := len(slugs) > limit
	if more {
		slugs = slugs[:limit]
	}
	items := []map[string]any{}
	var last publicArticle
	for _, slug := range slugs {
		a, err := s.publicArticle(r.Context(), slug)
		if err != nil {
			var known *Fault
			if errors.As(err, &known) && known.Status == 404 {
				continue
			}
			reply(w, nil, err)
			return
		}
		last = a
		items = append(items, map[string]any{"id": a.ID, "slug": a.Slug, "title": a.Title, "summary": a.Summary, "language": a.Language, "written_at": a.WrittenAt, "published_at": a.PublishedAt, "updated_at": a.UpdatedAt, "category": a.Category, "tags": a.Tags})
	}
	var next *string
	if more && len(items) > 0 {
		next = encodeCursor(last.PublishedAt, last.ID, filter)
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}
