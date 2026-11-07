# ADR 0009: English/Chinese public search

Status: Accepted and merged in PR #9.

## Decision

Search only the current published revision of public articles, including when the caller supplies an owner cookie. Read article metadata, labels, and Markdown together in one PostgreSQL statement snapshot after applying category/tag/language filters. Changes to publication, visibility, and label names are reflected on subsequent requests without a stale external index.

Queries require 1–200 Unicode characters and a non-whitespace term. Split on whitespace and require every distinct term to match somewhere in title, summary, tags, or body. ASCII letter/number/underscore terms match whole identifiers ignoring case, with ASCII identifier boundaries. Chinese and other non-ASCII or punctuation-containing terms match literal case-insensitive substrings. Percent signs and regex punctuation are literal input. There is no phrase-quote grammar, stemming, wildcard expansion, segmentation, typo correction, or semantic search.

Rank by the highest field level required to satisfy all terms: title (0), summary/tag names (1), then body (2). For example, terms split between title and body are ranked as a body match. Ties use first publication time and UUID descending. Bind cursors to the search format, normalized case/whitespace query, and active filters; changing page size is allowed. Ranking/filter changes between pages may change results; refresh for a current view.

Use the Markdown parser to extract text for body matching. Inline formatting does not break words, and fenced/inline code participates as ordinary body text. Exclude hidden HTML blocks, link/image destinations, comments, uploaded file contents, draft/history/private revisions, and category names. Search snippets are plain UTF-8 text of at most 240 characters plus ellipsis markers; body matches receive surrounding context. Rendering remains ordinary escaped React text.

## Execution and limits

For the first personal-blog release, stream the SQL result and match Markdown text in Go. Keep only the best `limit + 1` summaries in a bounded heap (default 20 results, maximum 100). Do not accumulate the full corpus or return raw Markdown in search responses. Reuse the ArticlePage contract without exposing internal scores.

Search work still scales with the amount of current public Markdown: there is no persisted text index and a smaller page size does not avoid the scan. The API's five-second public-request deadline bounds work, with context checks between documents/terms. Allow two active searches per API process; additional requests receive `503 search_busy` and `Retry-After: 1` so heavy searches cannot occupy every database connection. Measure with the real corpus before public deployment; introduce a maintained search projection/index when corpus size or measured latency warrants it. No new database schema or service is required now; version remains 4.

The homepage uses server-rendered GET forms for search and filters. Empty search, no matches, invalid input, and unavailable API states are distinct. Continuation links preserve the query and filters. Responses and fetches remain uncached. Interactive browser/visual review and production-scale performance acceptance remain pending.
