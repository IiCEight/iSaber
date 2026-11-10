# Home: continuous scroll study

Status: Local UI iteration in response to the owner's scroll-comparison screenshots. No remote push or CI/CD run is part of this study.

The initial Home confined its graphics to the opening hero and immediately entered the light article area. The reference screenshots instead keep graphics on screen while backgrounds, typography, and content panels change. This iteration carries the visual composition across four connected scenes:

1. Full-screen identity and a large loop of overlapping colored lenses.
2. A warm introduction composition: large “Keep exploring” typography, the same rotating loop, and a dark personal-introduction panel.
3. A blue composition with regrouped loops, “Stay curious,” and a direct invitation to the writing.

4. Recent writing enters the same blue canvas as the loops move toward its edges. Dark translucent article panels keep text readable while the graphics continue behind the list, archive invitation, and footer.

The article list remains in normal document flow within the shared story, so long summaries and larger lists expand naturally without a nested scroller or cropped content. Existing API data/empty/unavailable behavior is retained. No event videos, stock biography, sample articles, or separate project content type were introduced.

The hero's black backdrop slides completely past the right canvas edge during the introduction transition, rather than fading in place. Individual lenses breathe between 97.5% and 102.5% scale over staggered 5.6–7.2 second cycles. A nearby mouse gently attracts each lens toward the pointer, with an 28-pixel maximum screen displacement and a smooth return. Its influence extends 180 pixels past each lens’s visible radius, with a smooth outer falloff so large lenses respond consistently too. Attraction accounts for the parent loop's rotation and scale, so it remains bounded throughout the scroll sequence. Touch input does not attract the lenses; reduced-motion and linear fallbacks disable both effects. Pointer listeners and animation callbacks are removed when leaving Home or rebuilding its motion settings.

## Interaction

A native CSS sticky scene stays behind the entire story, including the article list and footer. A viewport-sized track sets the opening sequence length; GSAP ScrollTrigger scrubs the composition through the four scenes in either scroll direction. The reading entrance sets the main timeline endpoint independently of article-list height, and a separate gentle graphic drift continues through the reading section. The horizontal menu stays visible on Home. The graphic groups rotate around their centers so the loops remain in frame as their positions and scale change.

“Explore the writing” and “Read the latest” skip directly to the existing article section. Its heading has clearance below the fixed menu. The old `#home-title` fragment is normalized to the document-top `#content` anchor; a heading inside the sticky scene is not a stable top-of-page target. Footer navigation now uses that stable anchor too.

The complete linear content is the default HTML/CSS. Reduced motion, short viewports, or enlarged base text use that linear presentation without the sticky spacer. The reading section retains the blue palette and static colored lenses in these fallbacks. Live preference/font changes and route exits revert the scoped animation and remove the animated-layout class. JavaScript-disabled reading remains available.

## Local verification

Focused TypeScript/lint and Chromium checks cover the four scenes, persistent scene/menu bounds through a long list, direct article jumps and hash reloads, reduced motion, mobile overflow and keyboard visibility, enlarged-text fallback, route cleanup, and JavaScript-disabled content. Long-list QA uses temporary browser-only content without changing application data. Browser captures were inspected locally. The development server at `http://127.0.0.1:3000/` remains the review surface; no full production build or pipeline is needed for this iteration.

Pointer verification measures rendered bounding-box centers before and after mouse movement, including the first loop after its entrance and all three loops in scene three. Checking custom-property values alone misses transform overrides. The entrance animates a separate CSS variable so GSAP does not flatten the lenses' independent translate, rotation, and breathing scale into an inline transform.

This is a DOM/CSS interpretation of the supplied screenshots, not a reproduction of the reference site's WebGL shader. Final timing, copy, and graphic composition remain open to owner review.
