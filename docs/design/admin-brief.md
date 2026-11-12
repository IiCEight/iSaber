# Admin dashboard design

Status: The owner approved this design. It is now [integrated locally into the real admin dashboard](admin-integration.md), including the full shared Markdown preview. The prototype remains a visual reference. No push or deployment is part of this iteration.

## Direction

The owner requested conventional administration UI rather than the public site's artistic treatment. Use a dark navigation sidebar, a light workspace, white panels, restrained blue actions, readable forms, compact tables, and textual status badges. No decorative animation, oversized typography, graphical lenses, or analytics widgets are needed here. System fonts and native controls keep the prototype independent of external services and new dependencies.

Articles are the landing screen. The sidebar contains Articles, Import article, Categories & tags, and Comments. The header provides the workspace location, a link to the public-blog design, and sign-out. The sign-in study is deliberately separate from the signed-in workspace. On phones, navigation becomes a compact horizontal strip; forms stack, publication panels follow the editor, and wide data tables scroll within a labeled keyboard-focusable region.

## Screens and workflows

| Screen | Purpose and behavior |
| --- | --- |
| Articles | Search by title, switch All/Published/Drafts, filter visibility, reset filters, and open an article. Show title, category/language, unpublished changes, publication status, visibility, update date, and Edit. Counts describe the sample dataset only. |
| Import article | Import into a new private draft or a draft revision of the selected article. Show Markdown/ZIP limits, file selection/drop, a sample-import shortcut, and a three-step explanation. The live article stays unchanged by a revision import. |
| Article editor | Separate Metadata, Content preview, and Published versions views. Edit title, pre-publication URL, summary, language, writing date, category, and tags. Saving metadata is separate from publication. Show unsaved changes and confirm before leaving them. |
| Publication panel | Require a saved draft preview before enabling publication. Confirm publish/unpublish. Keep visibility and new-comment settings separate because they take effect independently of publishing. Show writing/publication/update dates. |
| Published versions | Read a historical version or restore it as a draft, explicitly retaining the live version until publication. Restoration invalidates the previous draft review. |
| Categories & tags | Create labels and rename their displayed names. Keep existing URL names and article associations fixed. No deletion control, matching the current backend. |
| Comments | Review Pending/Approved/Rejected/Removed queues. Confirm moderation and provide a link to the associated article. Approval only becomes publicly readable while the article is public and published; removed comments remain retained. |
| Owner sign-in | Conventional single-owner username/password layout with one primary action. The prototype explicitly warns against entering real credentials and sends/stores nothing. Production sign-in, session recovery, and sign-out continue to use the existing authentication implementation. |

## Prototype boundaries

Open [the prototype](http://127.0.0.1:8765/admin-prototype.html) using the repository's no-cache design server:

```bash
python3 docs/design/serve.py
```

All content and mutations are in-memory samples and reset on reload. The prototype makes no API requests, sends no credentials, and does not use local/session storage. Markdown files may be read locally to explore the import flow; contents are escaped before display. ZIP selection shows a sample result rather than claiming to parse the archive. The content preview is a layout study, not the production Markdown renderer.

The samples intentionally include English/Chinese articles, published/public, published/private, private drafts, unpublished changes, and several moderation states. They do not become production seed data or an authentication bypass. The public-page review fixes already in the working tree remain separate work.

## React integration after design review

Reuse `OwnerAccess`, `PublishingDashboard`, `LabelManager`, `RevisionHistory`, and `CommentModeration`. Apply the approved shell and screen organization while preserving the existing API request layer, session expiry, HttpOnly cookies, CSRF handling, idempotent retries, import polling/resume, version-conflict messages, public/private checks, immutable published history, and preview/publication rules.

Do not replace Typora with an unsolicited browser editor. Do not introduce reader accounts, multi-user permissions, delete controls, fabricated totals, or new backend endpoints just to populate the layout. Production article/label/comment lists use the existing cursors and loading/error/retry states; the small prototype lists are not a pagination specification. Authenticated content must remain absent from signed-out HTML and shared caches.

Review the navigation, table density, article editing hierarchy, import flow, and mobile layout before integrating this design into `/admin`.

## Local verification

Chromium review covered six screens at 1440, 1024, 768, 390, and 320 CSS pixels: 30 screen/viewport combinations without document-wide overflow or uncaught browser errors. Desktop/mobile WCAG A/AA scans passed for the six screens after contrast and status-tab fixes; the label dialog also passed its mobile scan. Screenshots were inspected.

Interaction checks passed for title search/reset, locked published URLs, unsaved-change cancellation, metadata save, preview gating, publication confirmation/cancellation, independent visibility changes, history read/restore, local Markdown import and escaped content, invalid-file recovery, label creation/rename, moderation queues/counts, and the sample sign-in transition. Keyboard checks covered visible focus after metadata saves and settings cancellation, native dialog Escape/focus restoration, and the separate signed-out screen. Network observation confirmed no API calls or non-GET requests during these sample workflows.

JavaScript syntax and Git whitespace checks passed. No application build, dependency installation into the project, or remote pipeline was needed for this static design study. Real API integration, cursor pagination, server validation, authentication, network failure/recovery, and live publishing acceptance remain the next implementation stage after visual review.
