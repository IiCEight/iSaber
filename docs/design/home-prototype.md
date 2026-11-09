# Vivid Home motion prototype

Status: The owner selected **A · Vivid**. The local motion prototype is ready for review before React integration or production deployment.

Open [the motion prototype](home-prototype.html) directly in a browser. Keep the adjacent JavaScript and `vendor/` directory with the HTML file. It uses local, pinned GSAP/ScrollTrigger files; there are no runtime CDN, font, image, or live-article requests. Sample content and navigation-to-design-notes remain clearly identified.

## Review controls

- **Replay entrance** returns to the hero and repeats the opening sequence. The status beside the controls explains why Replay is disabled when motion is reduced or a library cannot load.
- **Reduce motion** displays a complete static composition. When the OS requests reduced motion, the checkbox is checked and locked to reflect the effective setting. OS preferences always take precedence; changing the system animation setting updates this preview immediately. A manual preview preference is retained separately when the system setting changes.
- **Desktop / Tablet / Mobile** adjust the review frame. The frame never exceeds the actual browser window.
- **View full screen**, where supported, opens the page without the review toolbar and replays the entrance. Escape returns to the toolbar. Navigation to notes exits full-screen mode first.
- **Article state** retains four/single/empty/unavailable scenarios. No live data or API behavior changes.

## Choreography

Six circles enter through scale, rotation, and opacity with a short stagger. Identity typography follows with a vertical entrance. The sequence finishes in about one second; the header and writing link are immediately usable. Mobile uses a shorter graphic entrance.

Native scrolling translates and rotates the decorative wrapper slightly as the hero leaves view. The page is not pinned and scrolling is not replaced. The circle entrance and wrapper scroll transform animate separate elements to avoid conflicts.

Articles reveal as they enter view. Keyboard focus makes a row fully visible immediately, without waiting for its animation. The introduction and article content are ordinary HTML and remain visible if JavaScript or animation loading fails.

Animations are scoped through a GSAP context and reverted before responsive/state/full-screen rebuilds. The OS preference is observed while the page is open. Entrance motion pauses when the document is hidden. There is no continuous ambient animation loop. Page-hide cleanup reverts animations and disconnects the resize observer; back/forward-cache restoration reinstates the scoped setup.

## Dependencies and boundaries

GSAP/ScrollTrigger **3.15.1** were retrieved from the official npm registry and verified against its SHA-512 tarball integrity receipt. The original minified distributions retain their copyright/license notices; source URL, file SHA-256 values, and license URL are recorded in [vendor provenance](vendor/gsap-3.15.1/provenance.json).

This review artifact uses plain HTML to remain directly viewable. After motion approval, integrate the design into the existing React/Next.js application, install pinned packages through its package manager, scope animation lifecycle to React components, and connect the public article API while preserving Archive/Search and admin behavior. The production frontend has not been replaced or deployed by this prototype.

The circles are a DOM/CSS interpretation of the selected direction. This does not reproduce or verify the reference's WebGL shader. Lenis, Motion, and Three.js are not included; the current effects do not require them.

## Review questions

- Does the entrance feel appropriately paced?
- Is the scroll-linked movement too subtle, too strong, or comfortable?
- Does the transition from expressive hero to readable articles feel coherent?
- Do the portrait composition and reduced-motion alternative work for you?

## Browser verification

The prototype was exercised with a temporary Playwright 1.65.0 / Chromium headless-shell 158 browser. The real browser verified GSAP setup, actual scroll-linked graphic transforms, manual and OS reduced-motion changes, responsive rebuilds without growing trigger counts, article states, immediate keyboard-focused article visibility, mobile navigation, full-screen entry/exit and note links, and JavaScript-disabled/missing-library fallbacks. Direct `file://` opening worked with the local scripts. Checks at 360, 768, and 1440 pixels found no page-level horizontal overflow and no clipped headline bounds with 200% text sizing. No page errors were observed in those checks.

The test environment used a temporary Noto Sans SC font for Chinese coverage; it does not install or select production font assets. Review captures: [desktop hero](previews/home-vivid-desktop.png), [mobile hero](previews/home-vivid-mobile.png), and [article section](previews/home-vivid-articles.png).

A replay regression check verifies actual animation restart/completion, the manual reduced-motion toggle, the checked/locked OS-controlled state, restoration after live OS preference changes, preservation of manual preference, and missing-library messages beside Replay. This addresses the previously unexplained disabled button and unchecked checkbox when the system requested reduced motion.

This establishes Chromium functional/browser verification, not owner visual acceptance, Safari/Firefox coverage, assistive-technology acceptance, or target-device performance. Exact reference animation matching and target-device performance acceptance remain separate from selecting this prototype.
