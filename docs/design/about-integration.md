# About integration

Status: The approved animation study is implemented locally at `http://127.0.0.1:3000/about`. Nothing is pushed or deployed; no production build or CI/CD run is part of this iteration.

## Result

The server page renders the complete text, navigation, interest links, and placeholder disclosures. A scoped React client component uses the existing pinned GSAP/ScrollTrigger package to enhance the shared canvas. The pink band starts at full height on the right, fills the canvas in scene two, slides left to reveal blue in scene three, and exits completely in scene four. Reverse scrolling restores the same stages. Lens rearrangement, independent breathing, subtle bounded pointer movement, and row reveals match the prototype.

About uses the shared site header and navigation. Its header is sticky only on this route. Search opens Archive’s inline search, and all interest rows lead to the existing article collection rather than introducing separate content types or fake categories. Styles and animation selectors are scoped to About. Leaving the route removes GSAP state, listeners, observers, and pending anchor work.

A Reduce motion control is available in the hero footer; OS reduced-motion preferences always take precedence. Enlarged root text and short viewports use the complete linear layout. Hidden documents pause decorative breathing. Touch input does not trigger mouse parallax. JavaScript-disabled reading/navigation remains available, with no inert motion control shown. Short/zoomed content can grow naturally without getting trapped in a fixed crop.

The introduction is explicitly labeled draft copy, the personal biography is marked to be added, and optional contact placement is a plain note. No occupation, employer, location, credentials, portrait, email address, or profile was fabricated. The page does not depend on the local Go API. Owner-supplied biography and any public contact links remain content work before release.

## Verification

TypeScript and Biome passed. Browser checks covered actual pink-stage geometry, reverse scrolling, pointer displacement, manual/system reduced motion, section links and direct-fragment reloads, 1920/768/390/360-pixel layouts, enlarged text, short viewports, keyboard focus visibility, route cleanup, Archive search navigation, correct Home/Archive header behavior, and reading without JavaScript. Desktop/mobile captures were inspected. These are local browser checks; physical-device performance and the complete cross-page release review remain separate validation steps.
