// Package publishing implements the first Markdown publishing workflow.
package publishing

import "time"

type Issue struct {
	Severity string  `json:"severity"`
	Code     string  `json:"code"`
	Path     *string `json:"path"`
	Message  string  `json:"message"`
}

func issue(severity, code, path, message string) Issue {
	return Issue{severity, code, &path, message}
}

type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Revision struct {
	ID                   string     `json:"id"`
	ArticleID            string     `json:"article_id"`
	Title                *string    `json:"title"`
	Summary              *string    `json:"summary"`
	Language             *string    `json:"language"`
	WrittenAt            *string    `json:"written_at"`
	UpdatedAt            *time.Time `json:"updated_at"`
	Category             *Label     `json:"category"`
	Tags                 []Label    `json:"tags"`
	Markdown             string     `json:"markdown"`
	CreatedAt            time.Time  `json:"created_at"`
	FirstPublishedAt     *time.Time `json:"first_published_at"`
	RestoredFromRevision *string    `json:"restored_from_revision_id"`
	SourceIssues         []Issue    `json:"-"`
	Assets               []asset    `json:"-"`
}

type ArticleSummary struct {
	ID          string     `json:"id"`
	Slug        *string    `json:"slug"`
	Title       *string    `json:"title"`
	Status      string     `json:"status"`
	Visibility  string     `json:"visibility"`
	HasDraft    bool       `json:"has_draft"`
	Version     int64      `json:"version"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
	ModifiedAt  time.Time  `json:"modified_at"`
}

type Article struct {
	ArticleSummary
	CommentsEnabled bool      `json:"comments_enabled"`
	Draft           *Revision `json:"draft"`
	Published       *Revision `json:"published"`
	Publishable     bool      `json:"publishable"`
	Issues          []Issue   `json:"issues"`
}

type Job struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	ArticleID   *string    `json:"article_id"`
	RevisionID  *string    `json:"revision_id"`
	Publishable bool       `json:"publishable"`
	Issues      []Issue    `json:"issues"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

type Fault struct {
	Status int
	Code   string
}

func (f *Fault) Error() string            { return f.Code }
func fault(status int, code string) error { return &Fault{status, code} }

func blocking(issues []Issue) bool {
	for _, item := range issues {
		if item.Severity == "blocking" {
			return true
		}
	}
	return false
}
