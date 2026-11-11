# Product requirements: iSaber blog

Status: Approved first-release product baseline. The owner explicitly approved all documented defaults. Design references and numerical targets not yet specified remain open as listed in Section 9.

This document defines approved first-release behavior, incorporating requirements from `AGENTS.md` and the owner’s explicit approval of the defaults in this PRD. Product approval does not select unresolved technical implementations or constitute visual design approval.

Related documents: [development plan](../../DEVELOPMENT_PLAN.md), [repository decision](../architecture/decisions/0001-monorepo.md).

Approved implementation design: [data model](../architecture/data-model.md), [API design and requirement coverage](../architecture/api-design.md), and [OpenAPI contract](../../contracts/openapi.yaml). The owner approved the logical data model and API contract; they do not change this product baseline or imply an implemented server.

## 1. Purpose and outcomes

Create a personal publishing home for journals, technical articles, and portfolio content. The owner writes in Typora and publishes through a reliable Markdown import workflow. Visitors discover and read content through an artistic, distinctive interface that remains comfortable for long-form reading.

First-release success means:

- The owner can upload a representative Typora article with local images, preview it, and publish without manually repairing image URLs.
- English and Chinese articles, code blocks, and LaTeX math render correctly on desktop and mobile.
- Public visitors can discover articles through browsing, categories, tags, and search.
- Private material remains accessible only to the signed-in owner across every access path.
- Visitors can submit comments and the owner can moderate them.
- The owner approves the visual prototype and the final representative page layouts.

These are release acceptance outcomes. Traffic and engagement targets will be set after a usage baseline exists.

## 2. Users and permissions

| User | First-release capabilities |
| --- | --- |
| Visitor | Read public published content, browse and search, submit comments |
| Owner | Sign in, manage articles and metadata, preview and publish, read private content, moderate comments |

Confirmed owner decision: one owner account, no reader accounts, and no public registration in the first release. Private articles are readable only by the signed-in owner. Selected-reader access, memberships, and password-protected sharing are deferred.

### 2.1 Owner dashboard

The owner manages the blog at `/admin`, a protected section of the same React application that serves the public blog. The dashboard uses the same Go backend and storage as the public site. It does not require a separate management application, service, or deployment.

The public blog receives the artistic design treatment. The dashboard prioritizes clear navigation, efficient content management, and reliable feedback, while retaining accessible and responsive controls.

The dashboard provides:

- Article lists with search and draft/published and public/private filters.
- Markdown and image upload, metadata editing, and preview.
- Publication, unpublishing, visibility changes, and revision management.
- Category and tag management.
- Comment moderation and per-article comment controls.

Go verifies the owner session and authorization for every management operation. Protecting frontend routes or omitting navigation links alone is insufficient.

## 3. First-release scope

| ID | Capability | Scope |
| --- | --- | --- |
| AUTH-01 | Owner access | Initial owner setup, sign-in, sign-out, and a documented recovery process |
| ADMIN-01 | Owner dashboard | Protected `/admin` section in the React application, backed by the existing Go API |
| PUB-01 | Markdown import | Markdown file or ZIP with one article and its images |
| PUB-02 | Preview and publication | Drafts, preview, explicit publication, and unpublishing |
| PUB-03 | Article revisions | Re-upload into an existing article; preserve URL and prior published revisions |
| META-01 | Article metadata | Title, summary, slug, language, dates, category, tags, visibility |
| READ-01 | Reading experience | Markdown, code, images, links, tables, lists, and math |
| FIND-01 | Discovery | Article index, categories, tags, language filter, and search |
| PRIV-01 | Private content | Owner-only access across content and assets |
| COMM-01 | Comments | Guest submission, pending moderation, approval and removal |
| DESIGN-01 | Visual experience | Owner-approved artistic direction, responsive layouts, accessible interactions |
| PORT-01 | Personal identity and project writing | An introduction and project articles using the shared content model |

Owner-confirmed page refinement during Home design: public pages are Home, Archive, Article, About, and Search. Journals, technical writing, and projects are all articles, organized by owner-managed categories/tags. There is no separate Work page, project content type, or hardcoded Journal/Technical/Projects navigation. About provides the personal introduction. A dedicated portfolio editor remains outside scope.

## 4. Core journeys

### 4.1 Publish a new article

1. Owner signs in at `/admin` and selects a Markdown file or article ZIP.
2. The system validates the upload and reports import issues.
3. Owner reviews rendered content and supplies or adjusts metadata.
4. Owner selects public or private visibility and publishes explicitly.
5. The article becomes available to the appropriate readers.

### 4.2 Update an article

1. Owner opens an existing article and uploads a replacement.
2. The replacement becomes a draft revision; the published version stays available.
3. Owner previews the revision and explicitly publishes it.
4. The article URL and first publication time stay unchanged.

### 4.3 Discover and read

1. Visitor arrives at the homepage, article index, or a direct link.
2. Visitor browses or filters public articles, or searches by words or phrases.
3. Visitor opens an article, reads it on their device, and optionally submits a comment.

### 4.4 Keep a journal private

1. Owner uploads or edits an article and sets private visibility.
2. Owner can read the private article and its images while signed in.
3. Visitors cannot retrieve its content or discover its metadata through listings or search.

## 5. Functional acceptance criteria

### AUTH-01: Owner access

- Provision the single owner through a documented setup command; there is no browser-based registration flow. Re-running setup must not create another owner or silently replace credentials.
- Unauthenticated users cannot access administration or perform owner actions.
- Public visitors can browse and read without signing in; reader sign-in and registration are absent.
- Sign-out ends the active session. Expired sessions require sign-in again.
- Failed sign-in gives a useful error without exposing account details; repeated attempts are rate-limited.
- The owner has a documented recovery process before release. Login method and recovery mechanism require architecture review.
- A session expiring during upload or editing produces a recoverable state and never silently publishes or discards a previously saved draft.

### ADMIN-01: Owner dashboard

- `/admin` presents owner sign-in when unauthenticated and the management interface when authenticated. Management data is not included in unauthenticated page responses.
- The article list includes drafts, public articles, and private articles, with title search and filters for publication status and visibility. Empty lists, no matches, loading, and request failures have distinct states.
- From the dashboard, the owner can upload an article, edit metadata, preview, publish, unpublish, change visibility, and manage revisions as defined in PUB-01 through PUB-03.
- Category/tag management and comment moderation are reachable from dashboard navigation.
- Management actions use protected operations in the same Go API; there is no separate management backend or database. Direct unauthenticated API calls cannot read management data or perform mutations.
- Success and failure feedback identify whether an operation completed. Destructive or visibility-changing actions clearly identify the affected article and intended outcome before submission.
- The dashboard works on desktop and mobile with keyboard-accessible forms; artistic public-site animation is not a prerequisite for performing management tasks.

### PUB-01: Markdown and image import

- Accept UTF-8 `.md` files and ZIPs containing exactly one article and its relative image files. English and Chinese filenames are supported.
- Recognize relative image references, including reference-style images, and replace local paths with managed asset references in the imported content.
- A Markdown-only import with local images reports the missing files and offers re-upload as a ZIP. Browser access to arbitrary paths on the owner's computer is not assumed.
- Missing local images or invalid metadata block publication and identify the problem. Valid drafts may be retained for correction.
- External HTTPS image links remain external and are identified during preview; importing does not silently download remote content. Private articles show a notice that external images contact another host.
- Failed or interrupted imports cannot create a partially published article. Retrying does not publish or overwrite an existing article automatically.
- Reject unsafe archive paths, encrypted archives, unsupported file types, and excessive archive expansion. Limits: 5 MiB Markdown, 10 MiB per image, 50 MiB compressed ZIP, 100 MiB expanded ZIP, and 200 files.
- Import implementation defaults also bound decoded images to 16 million pixels, archive entries including directories to 400, and entry paths to 1024 UTF-8 bytes. Standard single-part ZIPs are supported; ZIP64, special files, and duplicate case-folded names are rejected.
- Initially accept PNG, JPEG, GIF, and WebP images. SVG uploads and general-purpose attachments are deferred pending a defined handling policy.

### PUB-02: Preview and publication

- Every new import starts as a draft. Saving or uploading alone never publishes.
- Owner-only preview renders the same content features as the published page.
- Publication validates required metadata and local assets, then makes a complete revision available together.
- Drafts are excluded from public pages, APIs, search, and any generated public indexes.
- Unpublishing removes public access to the article and its managed media, while preserving owner access and history.
- Success and failure states clearly indicate whether the article was published. A retry cannot create duplicate articles.

### PUB-03: Revisions

- Updating requires selecting an existing article; filename matching alone does not overwrite content.
- Re-uploading preserves the article identifier, slug, first publication time, and attached comments.
- The owner can preview a draft revision while visitors continue to read the last published revision.
- The owner can inspect prior published revisions and restore one as a new draft, then publish explicitly.
- Old revision content and media do not become publicly accessible merely because the current revision is public.
- Concurrent edits produce an explicit conflict rather than silently overwriting a newer change.
- Permanent deletion and retention rules are deferred; first-release content can be unpublished and retained privately.

### META-01: Metadata and dates

| Field | Approved behavior |
| --- | --- |
| Title | Required; supports English and Chinese |
| Summary | Optional; derive a plain-text excerpt if omitted |
| Slug | Required, unique, stable; editable before first publication, locked afterward in the first release |
| Language | Owner chooses English or Chinese; mixed-language article bodies are allowed |
| Category | Zero or one per article |
| Tags | Zero or more per article |
| Writing date | Owner-selected calendar date; required before publication, never inferred silently from the upload time |
| Publication date | Time of first explicit publication, preserved through revisions and republication |
| Update date | Last published content or metadata change; draft saves do not alter the public date |
| Visibility | Public or private; default private until explicitly chosen |

- Writing, publication, and update dates appear with distinct labels on article pages; identical timestamps may be visually combined only if meaning remains clear.
- Display timezone is Asia/Shanghai, shown where needed to avoid ambiguity. Writing date remains a calendar date.
- Optional front matter supplies metadata; the form displays parsed values before publication. Form corrections take precedence for that import. Invalid fields produce actionable errors.
- Owner can create categories and tags and assign them to articles. Renaming a label preserves article associations.
- Scheduled publication, backdated first-publication times, and slug redirects are outside the first release.

### READ-01: Content rendering

- Support headings, paragraphs, emphasis, links, ordered and unordered lists, task lists, blockquotes, tables, horizontal rules, fenced code, and images.
- Fenced code supports language-aware highlighting and preserves whitespace. Unknown languages remain readable as plain code.
- Support inline math using `$...$` and block math using `$$...$$`, documented with examples compatible with the agreed Typora workflow.
- Invalid math produces a readable fallback and preview warning without breaking the entire page.
- Raw HTML is escaped or removed; embedded scripts and executable Markdown extensions are not supported in the first release.
- Images preserve aspect ratio and support alternative text. Long code and formulas scroll within their own containers without overflowing the page.
- English and Chinese characters remain readable without missing glyphs. A fixture article covers mixed languages and punctuation.

### FIND-01: Browsing and search

- Public article lists show only published public articles, newest first by first publication time, with pagination or an equivalent accessible continuation control.
- Visitors can browse category and tag pages and filter by English or Chinese.
- First-release search covers title, summary, tags, and article text; excludes comments, code-only ranking enhancements, and file contents.
- English search supports case-insensitive word matching. Chinese search supports literal substring matching initially; segmentation, typo tolerance, and semantic search are deferred.
- Results show title, an excerpt, and a link. Ranking prioritizes title matches, then metadata, then body matches; ties use publication time.
- Search results and counts never expose private or draft content. The owner manages private content through administration, not public search.
- Empty queries, no results, and failed requests have distinct states. Empty queries prompt input rather than returning every article.
- Published changes appear on subsequent browsing and search requests; stale indexes or caches must not retain content after it becomes private or unpublished.

### PRIV-01: Private content

- Private articles and their managed assets require an authenticated owner session, including direct URL requests.
- Unauthorized requests do not reveal whether a private article exists; the public response is indistinguishable from a missing article.
- No private titles, excerpts, identifiers, or assets appear in public listings, APIs, search, feeds, sitemaps, page metadata, or shared caches.
- Changing public content to private applies to subsequent authorized-origin requests and invalidates public caches controlled by the application. Copies already downloaded or cached by third parties cannot be recalled.
- Media sharing between articles must not bypass these rules. Assets of private revisions cannot be exposed through another article's public asset route.

### COMM-01: Comments and moderation

- The guest form requires a display name and plain-text comment. Email, website fields, reader accounts, and nested replies are excluded initially.
- Submission confirms receipt and explains that moderation is pending. Pending and rejected comments are invisible to visitors.
- Owner can approve, reject, or remove comments. Approved comments appear chronologically on the associated public article.
- Owner can disable new comments per article. Private or unpublished articles accept no visitor comments and expose no comment history publicly.
- Renaming or revising an article preserves its comments. Comment text is never treated as executable markup.
- Limits: 80 characters for a name and 5,000 for a comment. Rate-limited or invalid submissions show a useful error without clearing the entered text.

### DESIGN-01: Artistic and usable interface

- Owner supplies references and approves the visual direction before full visual implementation.
- Prototype review covers Home, Archive, Article, About, Search, mobile navigation, and the owner upload flow.
- Typography, composition, colors, motion, and component states form a consistent design language rather than unrelated effects.
- Essential navigation and reading do not depend on hover, animation completion, or precise pointer movement.
- Keyboard users can reach interactive elements with a visible focus indicator; forms have labels and actionable errors.
- Reduced-motion preferences remove nonessential motion while preserving all content and controls.
- Verify representative layouts at 360, 768, and 1440 CSS pixels and with text enlarged to 200%. Article pages have no unintended page-level horizontal scrolling.
- Accessibility target: WCAG 2.2 AA. Agree on the detailed audit checklist during design review.
- Define measurable page-loading and interaction budgets against an agreed device/network profile before accepting expensive motion or media. Final performance thresholds remain an open decision.

### PORT-01: Personal identity and project writing

- Visitors can reach About and Archive from primary navigation.
- Project writeups use the same article model and reading page as other writing, with ordinary Markdown images/links and owner-managed category/tag labels.
- Home balances personal introduction with public writing; About can hold the longer biography and supplied contact links.
- Missing optional images or links do not create broken controls or empty blocks.
- Final copy, project content, imagery, and layout require owner review. Demo content must not be represented as the owner's actual work.

## 6. Reliability and operational acceptance

Owner scope revision, November 11, 2026: one Tencent deployment and public GHCR images through NJU replace separate cloud staging and automated backup/offsite/retention/recovery infrastructure. [ADR 0019](../architecture/decisions/0019-simple-delivery.md) records this revision. Functional privacy and persistence requirements remain.

- Saved drafts and published articles survive application restarts.
- Failed operations give the owner a clear next action; errors do not reveal credentials or private content.
- Preview and publication use the same rendering specification, verified against shared content examples.
- Database and managed media persist in private volumes across application updates. Automated backup transfers and replacement-server rehearsals are outside the revised scope.
- Local/CI acceptance uses disposable synthetic data and credentials; real private journals are not test fixtures.
- Browser coverage: current stable Chrome, Firefox, Safari, and Edge, plus iOS Safari and Android Chrome. Record versions used during release verification.
- Monitoring can identify failed publication, unavailable dependencies, and application errors without recording article bodies or comment text in logs.
- Agree on expected content volume, traffic assumptions, and performance targets before production sizing and release acceptance.

## 7. Explicitly deferred scope

Unless approved through a subsequent scope change, defer multiple authors, reader accounts and registration, selected-reader sharing, paid memberships, newsletters, automatic translations, collaborative editing, an in-browser rich-text authoring replacement for Typora, scheduled publishing, custom executable Markdown, advanced search, nested comments, a portfolio CMS, and a standalone media library. A separate management service or separately deployed admin frontend is also outside the first-release scope.

RSS and additional feed formats are optional follow-up features; privacy protections apply if they are introduced. Ordinary public page metadata and a public sitemap belong to release preparation.

## 8. Implementation slices

1. Owner setup and sign-in, the `/admin` dashboard, and one Markdown article through upload, preview, publication, and reading, including backend authorization from the outset.
2. Local-image packaging, import failures, revisions, metadata, and unpublishing.
3. Approved visual direction applied to real English/Chinese article fixtures and portfolio content.
4. Categories, tags, language filtering, and search.
5. Comment submission/moderation and comprehensive privacy regression checks.
6. Accessibility, performance verification, container acceptance, persistent production storage, public HTTPS, and application rollback.

The backend and design workstreams can progress independently against agreed contracts and realistic fixtures. Each slice receives acceptance criteria, focused changes, and review under the development plan.

## 9. Approved defaults and outstanding inputs

### Confirmed decisions

- One owner account; no reader accounts or public registration in the first release.
- Private articles are owner-only.
- A protected `/admin` section in the existing React application provides content management.
- Both public and management functions use the same Go backend and data stores.
- Owner provisioning uses a setup command rather than a registration page. Login and recovery are implemented for review in the [owner-authentication decision](../architecture/decisions/0005-owner-authentication.md).

### Approved defaults

The owner explicitly approved all defaults documented in this PRD, including the acceptance criteria and limits in Sections 3–8.

| Decision | Approved behavior |
| --- | --- |
| Comments | Guest name and plain text, owner moderation, no email or reader accounts |
| Publication | Upload creates a draft; explicit preview and publication |
| Uploads | One Markdown file or one-article ZIP with images, using the documented limits and file types |
| Updates | Explicit article selection, draft revisions, stable URL, and revision history |
| Metadata and dates | Documented field defaults, owner-selected writing date, automatic first-publication time, and public update date changed on publication; Asia/Shanghai display timezone |
| Chinese search | Literal substring matching for the first release |
| Personal/project content | Home/About introduction and shared article pages; owner-managed labels organize all subjects |
| Quality | Documented responsive and keyboard checks, WCAG 2.2 AA target, and browser coverage |
| Scope | First-release capabilities, deferred features, and implementation slice order as documented above |

### Outstanding inputs and technical decisions

Approval of defaults does not supply values or materials that have not yet been specified:

- Owner-selected design references, branding preferences, portfolio content, and representative articles are needed before visual direction approval.
- Numerical performance budgets, test device/network profile, expected content volume, and traffic assumptions must be defined before their respective design or release acceptance checks.
- Login, sessions, password storage, and owner recovery are merged. Production secrets, TLS hosting, ingress abuse controls, and other unselected deployment details remain to be configured.
- The logical data model and API contract are owner-approved, the implementation backlog is prepared, and PostgreSQL migrations are merged. Core article features are merged; final design/browser review and public release acceptance remain.

## 10. Review and approval

- [x] Owner confirms one account, no reader accounts, owner-only private content, and an integrated dashboard using the same Go backend.
- [x] Owner approves first-release scope and deferred features.
- [x] Owner explicitly approves all documented product defaults and acceptance criteria.
- [x] Functional requirements are translated into the [prioritized implementation backlog](backlog.md).
- [x] Owner approves the logical data model and API design, which reference the approved requirements.
- [ ] Design review uses real content fixtures and the owner-selected references.

Revisions to this PRD should record changed behavior and its impact on scope, contracts, design, and acceptance checks through focused pull requests.
