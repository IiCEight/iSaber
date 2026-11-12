# Archive integration

Status: The approved Archive design is integrated on the design branch at `/archive`. The owner subsequently pushed the branch and reported successful CI. The [November 12 local cross-page review](public-page-review.md) records current verification and open acceptance work. The standalone prototype remains a visual reference, with sample data isolated from the application.

## Result

The Archive carries Home's vivid masthead, breathing circular lenses, indigo reading area, and warm accents. The search icon expands a full-width section in normal document flow. Search, category, tag, and language conditions are stored in the URL and remain visible as removable chips after collapsing the controls. Editing fields does not change results until submission. Escape/collapse dismisses edits; reopening resets fields to the applied URL values. Search and browse pagination preserve conditions; changing conditions drops the previous cursor.

The shared server article browser retains the existing public API fetching and validation, then passes Archive data to a dedicated server view. A small client component handles disclosure, native GET form enhancement, pending feedback, and route navigation. The main Search navigation now links to `/archive#archive-search-panel`, revealing and focusing Archive’s inline search. Clicking Search while already on Archive preserves applied conditions; direct fragment links work as well. The existing `/search` route retains its behavior for older links; a separate Search-page design is deferred. Archive text searches use the public search endpoint, so their sort label says “Best match first”; ordinary browsing says “Newest first”. Page counts describe the returned page, not an invented collection total.

Without JavaScript, the full form remains visible and works through ordinary GET requests. Empty search text on Archive means browse, so native filter-only submissions still work. The separate Search page continues to prompt for an empty query. Unknown selected labels/languages remain representable; label failures do not discard applied conditions. Article outages, empty collections, no matches, and invalid input have distinct feedback. Decorative breathing and disclosure transitions respect reduced motion. Content and styles are scoped away from Home, Search, article reading, and admin.

## Verification

TypeScript and Biome checks passed. Browser integration checks used a temporary local HTTP fixture implementing the public API boundary; it was removed after testing. They covered real server rendering and navigation, applied conditions, draft edits, reopening, browser Back, cursor pagination/reset, unknown labels, unavailable labels, input validation, empty/error states, 1440/768/390/360-pixel layouts, enlarged text, reduced motion, native GET forms without JavaScript, Home/Search style isolation, and absence of reader-cookie forwarding or owner API requests. The existing production smoke suite was updated for the new structure and boundary regressions; its production-build run is deferred to the normal release workflow.

The local frontend is running on port 3000. The Go API is currently offline in this environment, so the normal preview shows the genuine article-unavailable state. No sample articles are injected into the live application, and no cloud data was accessed or changed.
