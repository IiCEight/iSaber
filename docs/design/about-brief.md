# About design review

Status: First local prototype, pending owner review. The live `/about` route is unchanged. No remote push or CI/CD is part of this iteration.

## Direction

About is the most expressive introduction after Home: giant typography, large flat blue/pink/indigo fields, a lens ring that loosens into a constellation as the visitor scrolls, and an ending that invites exploration. It complements Home’s circular visual language with a different choreography and keeps the calmer reading rhythm of Archive/Article out of this presentation page.

The page uses the established iSaber identity and the supplied description of a personal journal, technical articles, and things made, in English and Chinese. New copy is visibly marked as layout copy. No occupation, location, employer, portrait, work history, achievements, or social/contact accounts are invented. Contact placement is a design note, not a fake email button. Final biography and contact links require owner-supplied content before integration.

## Composition

1. A full-height typographic opening: “Curiosity, in motion.” The vivid ring occupies the other half; a clear link skips the opening.
2. A personal-note composition: a concise factual introduction in an indigo text block, with the ring opening into a scattered field. English and Chinese express the bilingual foundation.
3. Three large editorial interest rows—Notes, Code, Experiments—lead into the existing article collection. They illustrate the site's supplied purposes; they are not separate content types, portfolio routes, or fixed category tabs.
4. An oversized closing invitation to Archive, with space reserved for an optional real contact link.

The horizontal navigation matches the other prototypes. Search links to Archive’s inline search. Native section links and footer navigation remain available throughout. Graphics never intercept links.

## Motion

A scoped GSAP/ScrollTrigger timeline transforms the shared decorative canvas through the normal-flow chapters. No scroll hijacking or nested scroller is used. Lenses breathe independently while their outer slots rearrange and their overall rig rotates. A separate pointer layer adds a capped, mouse-only parallax without competing with scroll transforms. Interest rows get a restrained reveal and warm hover/focus treatment. Opening text enters once; content is never left hidden if animation setup fails.

The pink field is full height from the beginning, flush with the top, bottom, and right edges of the decorative canvas. It starts as a vertical band and expands only horizontally during the first transition. Scene two holds the full pink background. In scene three the full-width field slides left, revealing blue from the right. By scene four it has passed completely beyond the left edge and is hidden; its size never shrinks during this exit. Reverse scrolling restores the same stages. Text labels and copy retain opaque indigo fields for contrast. Static/reduced-motion layouts show pink for the introduction, a pink/blue split for the interests, and blue for the closing invitation.

The default document is a complete static layout. Reduced motion, short viewports, or enlarged root text disable sticky choreography and preserve the linear colored sections. A manual Reduce motion control helps compare the fallback; the OS preference cannot be overridden by the prototype. Preference changes revert GSAP context and clean up pointer listeners. Hidden documents pause decorative breathing; the continuous motion can also be disabled from the review control. No animation is required to discover a link or read a paragraph.

Preview: `http://127.0.0.1:8765/about-prototype.html`.

## Local verification

Chromium checks passed for opening and scrolled graphic states, sticky canvas bounds, reverse scrolling, native section links and reloads, focus visibility, 1440/768/390/360-pixel layouts, system/manual reduced motion, enlarged text, short viewports, and JavaScript-disabled reading. Separate checks measured actual bounded pointer-layer displacement and return to rest, and verified that individual lens breathing remains live. Desktop/mobile captures were visually inspected; a decorative marker that overlapped the small-screen scroll cue was removed. These are local browser checks, not a physical-device performance acceptance test.

## Reliable local preview

Run `python3 docs/design/serve.py` from the repository root. The local-only server disables cache storage and conditional 304 responses, so ordinary reloads fetch the latest study files. About’s CSS and script URLs also carry content versions to bypass assets cached by the previous server. This avoids mixing the old top/right offsets with the new full-height, full-width expansion.
