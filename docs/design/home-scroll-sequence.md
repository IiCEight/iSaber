# Home: continuous scroll study

Status: Local UI iteration in response to the owner's scroll-comparison screenshots. No remote push or CI/CD run is part of this study.

The initial Home confined its graphics to the opening hero and immediately entered the light article area. The reference screenshots instead keep graphics on screen while backgrounds, typography, and content panels change. This iteration carries the visual composition across three connected scenes:

1. Full-screen identity and a large loop of overlapping colored lenses.
2. A warm introduction composition: large “Keep exploring” typography, the same rotating loop, and a dark personal-introduction panel.
3. A blue composition with regrouped loops, “Stay curious,” and a direct invitation to the writing.

The article list follows in its readable light layout. Existing API data/empty/unavailable behavior is retained. No event videos, stock biography, sample articles, or separate project content type were introduced.

## Interaction

A native CSS sticky scene stays in view for approximately 3.4 desktop viewport heights (3 on mobile). GSAP ScrollTrigger scrubs the composition through those scenes in either scroll direction. The horizontal menu stays visible on Home. The graphic groups rotate around their centers so the loops remain in frame as their positions and scale change.

“Explore the writing” and “Read the latest” skip directly to the existing article section. Its heading has clearance below the fixed menu. The old `#home-title` fragment is normalized to the document-top `#content` anchor; a heading inside the sticky scene is not a stable top-of-page target. Footer navigation now uses that stable anchor too.

The complete linear content is the default HTML/CSS. Reduced motion, short viewports, or enlarged base text use that linear presentation without the sticky spacer. Live preference/font changes and route exits revert the scoped animation and remove the animated-layout class. JavaScript-disabled reading remains available.

## Local verification

Focused TypeScript/lint and Chromium checks cover the three scenes, persistent scene/menu bounds, backward scrolling, the direct article jump, the legacy fragment, reduced motion, mobile card bounds, enlarged-text fallback, route cleanup, and JavaScript-disabled content. Browser captures were inspected locally. The development server at `http://127.0.0.1:3000/` remains the review surface; no full production build or pipeline is needed for this iteration.

This is a DOM/CSS interpretation of the supplied screenshots, not a reproduction of the reference site's WebGL shader. Final timing, copy, and graphic composition remain open to owner review.
