# Article integration

Status: The approved reading design is integrated on the design branch at `/articles/[slug]`. The owner subsequently pushed the branch and reported successful CI. The [November 12 local cross-page review](public-page-review.md) records current reading fixes, production checks, and open acceptance work. The standalone prototype remains a visual reference.

## Result

The real public article supplies the opening title, summary, category, tags, language, and written/published/updated dates. A warm graphic opening transitions to a narrow indigo reading column. The contents rail is sticky on desktop and collapsible above the text on mobile; short articles without section headings use a single-column layout. All decorative movement respects reduced motion.

Contents are derived from the same Markdown syntax used for rendering, including GFM and math. Duplicate, Setext, inline-formatted, nested, and Chinese headings get unique namespaced anchors. Fenced code and raw HTML do not generate phantom entries. An in-body Markdown H1 renders as H2 so the article title remains the page's only H1. Directly used parser packages are declared at their already-installed, locked versions; the lockfile was validated offline.

Code highlighting and KaTeX remain server rendered. Code-copy buttons copy the rendered code's text, with honest clipboard feedback. Code and tables have labeled keyboard-focusable local scroll regions, preventing horizontal overflow of the whole page. Essential article text, links, approved comments, and contents remain available without JavaScript; clipboard and comment submission remain enhancements.

The shared Markdown renderer retains raw HTML restrictions, KaTeX trust/expansion limits, existing article-asset delivery, and external-image URL/referrer safeguards. Article-specific heading/copy/table enhancements are opt-in, leaving admin previews unchanged. No samples, cover-image requirement, reading-time claim, or social-profile data were added to the live page.

The existing comments component retains guest submission, owner moderation, rate limiting, text preservation, retry keys/idempotency, refresh/pagination, disabled comments, and older-backend compatibility. Comment storage failures still leave the article readable. Missing/private articles share a generic 404; article outages get a scoped retry boundary without exposing error details.

## Verification

Typecheck and Biome passed. Focused outline tests exercise parsing, duplicate/suffix collisions, and exact correspondence between navigation and rendering IDs. Browser checks with temporary local API fixtures covered actual Markdown/code copying, highlighted code, KaTeX, authorized image loading, blocked credentials in image URLs, raw HTML exclusion, contents anchors, comments awaiting moderation, rate-limit feedback, uncertain retries reusing the same key, disabled/empty/unavailable/missing comment endpoints, short/long/bilingual articles, 1440/768/390/360-pixel layouts, 200% base text, reduced motion, public reading without JavaScript, private-article HTTP 404 responses, and no reader-cookie forwarding to server public API requests. Missing-page UI and outage retry/recovery were checked with JavaScript enabled. Home and Search style isolation was checked after visiting Article. Desktop and mobile captures were inspected.

Release follow-up: the development server returns 404 with no readable fallback body when JavaScript is disabled on a missing-article request. Public article reading works without JavaScript. Recheck the missing-page visual fallback against the production server during release validation; no production-build verification is claimed here.

The temporary API fixtures were removed after checks. The local Go API is offline, so real article routes currently show the unavailable state until that API is running. The visual prototype remains at `http://127.0.0.1:8765/article-prototype.html`. No cloud data or databases were changed.
