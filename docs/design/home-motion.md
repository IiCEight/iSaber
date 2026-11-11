# Home page reference and motion plan

Status: Owner selected **A · Vivid** and positively reviewed the [GSAP prototype](home-prototype.md). The [React integration](home-integration.md) and [continuous scroll work](home-scroll-sequence.md) merged in PRs #22–23 and are live, with real public articles and preserved Archive/Search routes. Final visual/browser acceptance remains open.

## Reference evidence

The owner supplied [C2 Montréal](https://c2mtl.koki-kiko.com/) as the first reference. Page and shipped-script inspection found WebGL canvases for logo/footer graphics, GSAP/ScrollTrigger, Lenis references, parallax components, split-text components, and custom-cursor components.

These findings establish implementation techniques. They do not establish how every effect looks in motion. Interactive visual playback was unavailable during inspection, so colors, camera movement, precise sequencing, scroll distances, and easing are not recorded as visually verified observations. A recording or interactive inspection should close those gaps before claiming a close reproduction.

The owner subsequently supplied a screenshot and specified a full-screen hero. The image establishes the opening composition: broad warm/cool color blocks, large overlapping circular graphics across their boundary, oversized typography near the lower edge, and a narrow navigation rail at the right. This is evidence of one visual state, not animation playback. The revised wireframe uses that composition in grayscale; final color and graphic treatment remain for visual-direction review. The original two-column hero proposal is superseded.

The owner approved the full-screen composition and requested horizontal navigation at the top. The blog therefore adapts the reference with a top bar rather than its side rail. The bar overlays the opening composition, with space reserved beneath it for content; the hero including navigation remains one viewport tall at minimum.

Use the reference's expressive motion as a design input for an original personal blog. Translate relevant patterns into the approved article model and owner content. The event's ticketing, speaker, and marketing content are not part of this blog brief.

## Proposed prototype interactions

| Interaction | Purpose | First prototype | Mobile/reduced-motion behavior |
| --- | --- | --- | --- |
| Introductory sequence | Establish identity and a distinctive opening | Coordinate wordmark/headline and one decorative element; keep navigation immediately usable | Shorter movement on mobile; static final composition with reduced motion |
| Article entrance | Lead attention from identity to writing | Small vertical reveal and opacity transition when rows enter the viewport | Gentle or static; links remain available throughout |
| Article hover/focus | Communicate that the title opens an article | Underline or small directional indicator with a matching keyboard focus treatment | Clear tap target; no reliance on hover |
| Section progression | Connect introduction and recent articles | Prototype one restrained scroll-linked transformation | Natural document flow; remove parallax under reduced motion |
| Navigation/search | Keep actions predictable | Native links and clear control feedback; prototype overlay only when its interaction is designed | Visible focus, correct focus management, and escape/close behavior if an overlay is used |

Starting timing ranges for testing: 400–700 ms for a hero element, 40–70 ms stagger where useful, and 150–250 ms for small control feedback. Aim to finish the initial sequence within roughly 1.2 seconds. These are proposed tuning values, not measurements of the reference. Clicking and reading must not wait for the sequence to finish.

Choose one signature effect within the full-screen hero for the first motion prototype. Prototype layered circular geometry sharing the canvas with typography. A decorative WebGL version can be evaluated if simpler graphics cannot express the approved direction. A full-screen section does not itself require scroll pinning; pinning, smooth scrolling, and a custom cursor each need a concrete purpose and mobile/accessibility review before inclusion.

## Tool responsibilities

- CSS owns ordinary hover/focus states and simple transitions.
- GSAP with ScrollTrigger owns coordinated timelines and scroll-linked sequences. Use React-scoped setup and cleanup; revert timelines/listeners when components unmount or responsive conditions change.
- Motion remains an option for a specific component-state or shared-layout interaction. Avoid having two systems animate the same properties on the same element.
- Keep native scrolling initially. Evaluate Lenis only if the approved prototype benefits from it while preserving anchors, keyboard scrolling, restoration, and touch behavior.
- Add Three.js only for an approved WebGL effect. Load decorative graphics separately and provide a meaningful static fallback when graphics cannot run.

References: [GSAP React integration](https://gsap.com/resources/React/), [ScrollTrigger](https://gsap.com/docs/v3/Plugins/ScrollTrigger/), [responsive animation setup](https://gsap.com/docs/v3/GSAP/gsap.matchMedia%28%29/), [Motion layout animation](https://motion.dev/docs/react-layout-animations), [Motion accessibility](https://motion.dev/docs/react-accessibility), [Lenis](https://github.com/darkroomengineering/lenis), [Three.js](https://threejs.org/manual/pages/creating-a-scene.html).

## Acceptance criteria

- Public content and navigation remain readable and usable without animation or JavaScript; decorative code failure must not conceal the page.
- Reduced-motion settings produce a complete static or restrained experience, including after a preference change.
- No animation blocks the primary article action, traps scrolling, or requires pointer precision.
- Transforms and opacity are preferred for DOM movement; observe layout stability and frame pacing on the agreed test devices.
- Repeated route visits and resizes do not accumulate timelines, canvas render loops, or event listeners.
- Decorative rendering pauses when offscreen or the document is hidden; any WebGL prototype bounds its rendering resolution and releases resources on unmount.
- Verify keyboard, touch, focus visibility, long English/Chinese titles, and mobile viewport changes.
- Compare the prototype with visual reference evidence and record intentional adaptations before implementation approval.
