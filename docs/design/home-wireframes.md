# Home wireframe review

Status: Owner approved the full-screen composition with horizontal top navigation. Continue with the [visual-direction comparison](home-directions.md) before animation work.

Open [the standalone wireframe](home-wireframes.html) in a browser. It runs directly from the file with no package installation, server, network requests, or access to live data. Desktop, Tablet, and Mobile controls change the frame width; a state selector previews four articles, one article, an empty list, and an unavailable API. Frame widths never exceed the actual window, so use a desktop browser to compare larger sizes. Without JavaScript, the default responsive four-article composition remains readable and mobile navigation still uses native disclosure controls.

The wireframe deliberately uses grayscale and system fonts. Its full-screen hero follows the owner's screenshot through shared typography/graphic space, overlapping circles, and block geometry. The owner approved this composition and requested a horizontal menu bar at the top, replacing the proposed side rail. The final palette and graphics remain for visual-direction review. Article and biography copy are marked samples. Navigation goes to explanatory notes rather than pretending that additional pages have been implemented. Where supported, **View full screen** removes the surrounding review interface; press Escape to leave that browser mode.

## Proposed layout

| Area | Desktop | Mobile |
| --- | --- | --- |
| Hero/navigation | Full-screen shared canvas, overlapping shapes, oversized lower-edge typography, writing link, horizontal top navigation | Full-screen canvas recomposed to a portrait format; compact top menu and visible writing link |
| Introduction | Short personal text below the hero, with About link | The same text in normal document flow |
| Recent writing | Editorial rows, titles/summaries, optional category, language, publication date | Single-column rows with natural title wrapping and the same metadata |
| Archive invitation | One compact invitation after the article list | Stacked naturally if needed |
| Footer | Identity and reserved contact area | Flexible wrapping |

The hero has a minimum height of one viewport (`100svh`), with content growth allowed for enlarged text. Identity and graphics share a canvas rather than separate columns. A writing link jumps directly to the article section; no animation must finish before navigation works. The introduction and recent articles follow beneath the opening screen. In framed review mode, viewport height follows the actual browser, not a simulated phone's physical height. A row-based article list accommodates English/Chinese titles, a missing summary/category, and one article without empty card slots. Article cover images are not required by this structure.

## Decisions for review

- Does the balance of introduction and writing feel right?
- Is the article-row layout appropriate, or should a later visual direction make one article more prominent?
- Does the portrait hero retain its impact on mobile with the compact top menu?
- Does the bottom archive invitation add value beyond the header/section link?

After structure approval, explore two visual directions using this same content hierarchy and then develop the chosen motion prototype. Keep production routes, search/filter functionality, and owner dashboard behavior unchanged during this review.

## Verification

The artifact is self-contained and uses no external assets or dependencies. Static checks verify unique anchor targets, navigation references, and the article/viewport controls. Interactive browser inspection, screenshots, 200% zoom, and keyboard/touch review still need visual browser verification; do not treat static checks as that acceptance.
