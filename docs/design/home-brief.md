# Home page design brief

Status: Owner approved the purpose and content hierarchy, then specified a full-screen hero using the supplied C2 Montréal screenshot as the composition reference. Wireframes are being revised accordingly; visual direction and motion prototype are not yet approved.

## Agreed foundation

- Balance introducing the owner and showcasing their writing.
- Treat all entries as articles. Categories and tags organize subjects; journal, technical writing, and projects are not separate content types or fixed navigation tabs.
- Keep Home, Archive, Article, About, and Search as the public page structure. Search may become an overlay with a shareable results route. The owner dashboard remains `/admin`.
- Use the owner's selected [C2 Montréal reference](https://c2mtl.koki-kiko.com/) to investigate expressive composition and animation.
- Use the existing React/Next.js frontend and Go API. Begin animation implementation with CSS and GSAP/ScrollTrigger. Motion, Lenis, and Three.js remain optional tools justified by a specific approved interaction.

## Audience, purpose, and success

Serve first-time visitors discovering the owner and returning readers looking for articles. A first-time visitor should quickly understand whose site this is, what they can explore, and how to begin reading. Returning visitors should reach recent writing directly.

Primary action: open an article. Secondary actions: browse the archive and learn about the owner. Home should express personality while making writing visible and easy to reach.

During prototype review, check whether visitors can identify the site owner, find the newest article, and open the archive without waiting for an animation or discovering a hidden interaction. Numerical performance budgets will be agreed against a test device/network profile before accepting expensive effects.

## Proposed content order

| Area | Content and purpose | Visitor action |
| --- | --- | --- |
| Hero and navigation | Full-screen canvas with oversized identity typography and overlapping graphics; horizontal top navigation on desktop and compact top navigation on mobile | Navigate or continue to articles immediately |
| Introduction | Short personal introduction below the opening canvas | Learn about the owner |
| Recent articles | Up to four public articles with title, short summary, category when present, language, and publication date | Open an article |
| Explore | A clear archive link; optional links to actual public categories if they help discovery | Browse all articles or a category |
| Footer | Compact site identity and owner-supplied contact/social links | Contact or return to navigation |

The hero fills the opening viewport. Typography and graphics share its composition, following the owner's screenshot; a separate text column and boxed illustration no longer define this layout. A visible writing link lets readers skip directly to recent articles. The introduction and articles follow below, and navigation remains usable without waiting for animation. Full-screen sizing must allow content to grow when text is enlarged rather than clipping controls.

Avoid repeating the biography in another large section. About can hold the longer personal story. Category links should add useful navigation rather than repeat every label in the header and footer.

## Content and data rules

Use the existing public article API and public label endpoints. The initial article selection is the latest public writing returned by the API. The current model has no editorial-featured flag or cover-image field: prominent layout does not imply a new selection mechanism or require images on every article.

Do not fabricate biography, professional credentials, projects, social profiles, article counts, or readership statistics. Use the established iSaber name provisionally; the owner reviews the final display name, introduction, and contact links. Prototype article fixtures must be visibly identified as samples rather than presented as the owner's published work.

Handle one article as a complete composition, rather than leaving empty card slots. With no articles, show a quiet, honest empty state. If the API fails, preserve the introduction/navigation and show a distinct retry message. Missing categories or summaries must not leave empty visual blocks. English and Chinese titles, long titles, and mixed-language text must fit naturally.

Only public articles and public labels may appear. Private articles, drafts, and owner metadata must remain absent from the page and its responses.

## Responsive and accessibility requirements

Review at 360, 768, and 1440 CSS pixels and at 200% text zoom. Mobile keeps the same content priorities, with a readable single-column article flow and comfortable touch targets. Essential content and actions must work with a keyboard, touch input, JavaScript unavailable, or reduced motion enabled.

Maintain visible focus, semantic headings, adequate contrast, and the existing skip link. Text animation must preserve the accessible reading order and avoid duplicate screen-reader announcements. Decorative graphics must not intercept article links or navigation.

See the [motion plan](home-motion.md) for interaction rules and the limits of the reference inspection.

## Implementation boundaries

The current `/` page also implements browsing, filtering, search, and pagination. When introducing the new Home, preserve those journeys through Archive/Search and maintain existing article links. Route migration is a separately verified implementation task; the prototype must not silently remove those capabilities or introduce dead navigation.

Keep the admin dashboard and article reading styles stable while exploring Home. Animation code should be scoped to the relevant client components, with content/data fetching staying in the existing server-rendered structure where practical.

## Review sequence and open inputs

1. Review this brief and proposed content hierarchy.
2. Record the reference breakdown, including visual playback evidence when available.
3. Produce desktop/mobile wireframes using realistic content.
4. Explore two visual directions against the same hierarchy and choose one.
5. Build an interactive prototype of the chosen direction, testing the signature animation early.
6. Implement the approved experience and complete responsive, accessibility, privacy, and performance checks.

Still needed: final display name and introduction, contact/social links, representative English/Chinese writing, and confirmation of which reference interactions matter most. The owner supplied a static screenshot showing the opening composition, supplementing the page/source inspection. Interactive visual playback was unavailable in this session. Exact animation timing and choreography remain to be validated through a browser session or owner-provided recording.
