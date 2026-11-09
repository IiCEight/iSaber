# Home wireframe review

Status: The brief and content hierarchy are owner-approved. These wireframes are proposed for review before visual direction and animation work.

Open [the standalone wireframe](home-wireframes.html) in a browser. It runs directly from the file with no package installation, server, network requests, or access to live data. Desktop, Tablet, and Mobile controls change the frame width; a state selector previews four articles, one article, an empty list, and an unavailable API. Frame widths never exceed the actual window, so use a desktop browser to compare larger sizes. Without JavaScript, the default responsive four-article composition remains readable and mobile navigation still uses native disclosure controls.

The wireframe deliberately uses grayscale and system fonts. The neutral graphic is a placeholder for the visual-direction review. Article and biography copy are marked samples. Navigation goes to explanatory notes rather than pretending that additional pages have been implemented.

## Proposed layout

| Area | Desktop | Mobile |
| --- | --- | --- |
| Header | Wordmark at left; Archive, About, Search at right | Wordmark and a compact native menu |
| Introduction | Identity/headline at left, graphic area at right; visible writing link | Headline, short introduction, writing link, then a compact graphic |
| Recent writing | Editorial rows, titles/summaries, optional category, language, publication date | Single-column rows with natural title wrapping and the same metadata |
| Archive invitation | One compact invitation after the article list | Stacked naturally if needed |
| Footer | Identity and reserved contact area | Flexible wrapping |

The hero has content-driven height rather than occupying the whole viewport. Its purpose is to introduce the owner while keeping recent articles nearby. A row-based list accommodates English/Chinese titles, a missing summary/category, and one article without empty card slots. Article cover images are not required by this structure.

## Decisions for review

- Does the balance of introduction and writing feel right?
- Is the article-row layout appropriate, or should a later visual direction make one article more prominent?
- Does the graphic deserve this much space, especially on mobile?
- Does the bottom archive invitation add value beyond the header/section link?

After structure approval, explore two visual directions using this same content hierarchy and then develop the chosen motion prototype. Keep production routes, search/filter functionality, and owner dashboard behavior unchanged during this review.

## Verification

The artifact is self-contained and uses no external assets or dependencies. Static checks verify unique anchor targets, navigation references, and the article/viewport controls. Interactive browser inspection, screenshots, 200% zoom, and keyboard/touch review still need visual browser verification; do not treat static checks as that acceptance.
