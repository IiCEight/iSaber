# Archive design review

Status: Layout approved and [integrated locally](archive-integration.md) for review. The standalone prototype remains a reference. No push or CI/CD is part of this iteration.

## Purpose and direction

Help readers find and open an article, with categories and tags organizing all writing. Continue Home's vivid blue, warm gradients, overlapping circular lenses, oversized typography, and horizontal navigation. Give browsing a quieter rhythm: a compact graphic masthead, followed immediately by a readable index with a search icon beside “All writing”. No scroll-pinned sequence or forced animation before the articles.

Desktop: a two-column masthead pairs the title/introduction with a circular graphic. Search and category/tag/language filters are hidden initially. A search icon expands a full-width blue-violet section below “All writing”, pushing the articles down in normal flow. No floating frame, scrim, blur, or scroll lock is used. A short vertical reveal is disabled under reduced motion. Pink/coral accents and a slim search field carry the palette into the section. The icon exposes its expanded state, closed controls are inert, and Escape or the collapse button returns focus to the icon. Submitting applies the conditions and collapses the controls; dismissing without submitting preserves the previous applied conditions. Removable chips for search, category, tag, and language remain visible above the result count while collapsed, with a Clear all action. Removing a chip updates results immediately and preserves keyboard focus. Reopening restores applied values. Repeated collection copy is removed. Each row has a publication date, title, summary, category, tags, and language, with larger metadata for easier scanning. The arrow and title open the same article. A coral hover accent connects the reading area to Home's warm palette.

Mobile: retain the same reading order with a smaller graphic, stacked filters, date above title, and wrapping metadata. No horizontal scrolling or hidden essential filters. The masthead grows with content and enlarged text.

## Prototype behavior

Open `archive-prototype.html` through a local static server. All article content is explicitly sample data for design review; counts describe only that fixture set. Category, tag, language, and text controls filter those samples. This local search is a preview interaction, not a specification for replacing the real Go search behavior. Clear restores the collection; no matches has a reset action. Article links open a clearly labeled sample preview dialog, not fictional published routes. Home, Archive, and About navigation link to the existing local frontend; Search opens the same inline section. A review control switches between sample, empty, and unavailable states. Retry returns to the sample state without claiming to contact a backend.

Soft independent lens breathing is decorative, with no pointer attraction or scroll choreography in this first Archive pass. Reduced motion disables it. Native form controls, keyboard focus, semantic landmarks, and a dismissible sample dialog support review without mouse input. No JavaScript still exposes the sample rows and an explanatory notice.

## Integration requirements

The React integration connects the approved structure to the existing public article and label endpoints. Keep GET filter URLs, search semantics, cursor pagination, clear actions, unknown selected labels, unavailable-label feedback, and privacy boundaries. Do not ship these samples or the review controls. Preserve the separate Search page until its own design review.

Review the hierarchy, balance of graphics and writing, row density, and mobile filter layout before refining motion or implementing the production route.

## Local review checks

Chromium checks passed for default-collapsed/inert controls, inline expansion, applied-condition chips and removal, draft versus applied values, focus restoration, Escape/collapse, sample category/language combinations, search, no-results reset, empty/unavailable states, retry, sample dialog and Escape, 360/390/768/1440-pixel layouts, 200% base text, and reduced motion. Desktop/mobile captures were visually reviewed. These checks verify the prototype; production API integration remains a later step.
