# Category page design review

Status: Visual design approved. Category pages, owner-editable bilingual descriptions, independent category pins, and migration 00009 are now implemented locally. The standalone prototype remains available for design reference. Nothing has been pushed or deployed.

## Approved behavior

- Each category gets a dedicated `/categories/{slug}` page with a name, description, and article collection. Existing category labels will link to these pages during integration.
- Category descriptions are owner-editable and support English and Chinese. The prototype's short bilingual description is sample copy.
- An article has independent global and category pins. A category page promotes its category pins; Home and Archive retain global ordering. Category filters still constrain all results.
- Published private articles and their category pins remain visible only to the authenticated owner. Public responses must not reveal private articles, counts, or private-only category metadata. The prototype uses public sample data only and does not simulate authorization.
- Publishing into another category or unpublishing clears the category pin. Pin changes preserve writing, publication, and content-update dates.
- Search keeps the existing Archive semantics, including all-term matching and the selected date/relevance order. Category pins are marked, without boosting search results.

## Visual composition

The black horizontal navigation remains. Direction 03 uses one dominant violet color field with a restrained tonal gradient, large sans-serif category title, bilingual description, and a quiet count/language line. Four overlapping circles form an arc along the right edge, with an enlarged warm circle mostly outside the upper-right corner so only its curved segment is visible. The owner-adjusted circle placement is preserved. A related plum, rose, coral, and apricot palette gives each circle a slightly different hue; the upper-right arc is the softest warm accent. Their gradients and staggered breathing give the geometric field variation without a flower motif or a separate graphic card. The header has no article jump link. On mobile, the circle group sits below the description, with the fourth circle mostly beyond the right edge. Only the masthead's outer boundary clips the artwork; the shorter art layout box does not truncate circle edges above the metadata strip. Shapes remain clear of text. No full-screen or scroll-pinned scene delays browsing.

The reading section uses the selected `#FFE2ED` light palette. It reuses the actual application's Archive, pagination, theme, and foundation CSS through a fixed allowlist in the local design server. Dark mode changes reading surfaces while preserving the violet category header and black navigation. The prototype's theme preference is stored under its own local-storage key and does not change the application's cookie.

Rows retain the Archive's title/summary/date rhythm. Category pins say **Pinned in Technology**, with a solid violet rectangular background, white text, and the pin icon. Dark mode uses a rose background and navy text to keep the badge distinct. The page does not repeat a Technology badge on every article. A globally pinned fixture remains in normal category order and has no category-pin badge.

A search icon expands the existing flat inline panel. Technology remains the page's fixed category; only text, tag, and language conditions are editable. Applied conditions stay visible as removable chips while the panel is collapsed. Pagination uses ten articles per page and preserves active conditions and sorting in the URL. The category's overall count remains distinct from the filtered result count.

## Prototype scope and review

Run from the repository root:

```bash
python3 docs/design/serve.py --port 8765
```

Open <http://127.0.0.1:8765/category-prototype.html>. Review controls outside the product page switch between sample, empty, and unavailable states. All 12 articles and the description are explicitly sample content. Sample search covers titles, summaries, and tags; production integration will use the Go API's full search. Article titles open a labeled sample preview, rather than claiming to be real published articles. No accounts, API calls, real pin mutations, or production changes are made.

The reduced-motion control follows the system initially and permits an explicit animation preview. Staggered circle breathing, a small vertical drift, and the title entrance are decorative. They do not depend on scroll. Row reveals never gate access; focused rows become fully visible. Reduced motion disables these effects, and off-screen or background graphics pause. Without JavaScript, all sample articles remain readable and unavailable interactive controls stay hidden or disabled.

Review the category masthead, description spacing, pin labels, reading density, filter panel, and mobile hierarchy before integrating the production routes and controls.

## Verification

Fourteen Chromium interaction checks passed: category pin/date ordering, independent global pins, paging and history, displayed date changes, inline filters and draft dismissal, condition removal and focus, all-term sample search and relevance, query escaping/no matches, empty/unavailable states, sample-dialog keyboard access, theme persistence/scroll, reduced motion with explicit preview opt-in, responsive/long-title layouts, enlarged text/mobile search, and the no-JavaScript fallback. There were no page errors or unexpected asset failures.

Layouts were checked at 360, 390, 768, and 1440 CSS pixels, including a long title on page two and enlarged base text. Header direction 03 is also reviewed at 1900 pixels, including text/shape separation and the metadata strip. Automated WCAG A/AA checks reported no violations for light desktop, expanded search, dark desktop, and light mobile states. The prototype uses no external requests. The fixed shared-style aliases return public assets only; an arbitrary `/shared/.env` request returns 404.

The four-circle refinement was checked at 360, 390, 620, 768, 1440, 1600, and 1900 pixels across six animation phases. The art container has no inner clipping; rounded lower edges remain above the metadata strip and clear of the title and mobile description. The fourth circle follows the reduced-motion control.

Saved captures:

- [Single-color circle header, wide desktop](previews/category-header-desktop.png)
- [Desktop category page](previews/category-desktop.png)
- [Applied conditions remain visible](previews/category-filtered-desktop.png)
- [Mobile category page](previews/category-mobile.png)

The application implementation reuses Archive filters, sorting, pagination, and reading permissions. Category metadata and numbered article endpoints exist in public and protected owner-reading scopes. Categories without public published articles return 404 to guests; owners can view empty categories and private publications. Migration 00009 is verified in an isolated test database and applied to the persistent local development database with a backup and unchanged data fingerprints. Cloud databases remain unchanged. The integrated local app now runs at http://127.0.0.1:3000 against the persistent database, which currently has no owner or articles; the isolated sample preview was stopped.
