# Vivid Home: React integration

Status: Vivid Home merged in PR #22, followed by continuous scroll/lens interactions in PR #23. The owner selected Vivid and positively reviewed the prototype. The integration is live on Tencent; final copy/fonts, remaining page design and real-content browser acceptance remain open.

## Result

The full-screen Home includes horizontal top navigation, overlapping decorative circles, large identity typography, a writing link, introduction, and up to four recent public articles. Data comes from the existing Go public article endpoint. Empty and unavailable responses have distinct states; no demonstration articles, biography credentials, or fictional contact links are shipped as production content.

All subjects use the same article model. Category links lead into Archive. The current public-summary model needs no cover-image or featured-article field, and this milestone adds no backend API, database schema, or storage changes.

## Routes and compatibility

| Route | Behavior |
| --- | --- |
| `/` | Approved Vivid Home, introduction, latest public writing |
| `/archive` | Existing article browsing, category/tag/language filters, and cursor pagination |
| `/search` | Existing public search behavior, validation, filters, and result pagination |
| `/about` | A short introduction grounded in the approved site purpose; expanded biography remains owner content |
| `/articles/{slug}` | Existing shared reading page, with category/tag links now pointing directly to Archive |
| `/admin` | Existing owner access and publishing dashboard |

Legacy `/?q=...` links redirect to Search. Root URLs containing category/tag/language/cursor parameters redirect to Archive. Redirects retain repeated query values so malformed search inputs continue through the existing validation path. Unrelated tracking parameters do not replace the new Home with an archive view. Article URLs are unchanged.

The header uses native mobile disclosure navigation, closes on navigation/Escape, and works without JavaScript. The skip link stays above the header. Footer and header links reach actual routes; prototype controls and explanatory navigation destinations do not appear on the production page.

## Animation implementation

GSAP 3.15.1 is installed through the frontend package manager and lockfile. A small client boundary scopes the animation around server-rendered content. Scoped contexts and media listeners are reverted/removed on cleanup; reduced-motion and mobile media-query changes rebuild the animation without accumulating independent timelines.

The hero entrance and scroll transform target separate elements. Native scrolling remains intact. Article focus completes its reveal immediately. OS reduced motion produces the complete static page. Animation setup failure reverts animated styles so content remains available. The production page does not include a Replay button, test-state selector, or an OS-preference override.

Home layout/styles are scoped so owner forms, article Markdown, code, math, and comments retain their existing behavior. The linter now covers components and API helpers alongside pages. Direct Markdown images remain deliberate: image optimization must not cache private media or fetch arbitrary external URLs.

## Verification and review boundary

Production HTTP tests exercise SSR/no-store behavior, four-summary rendering, Unicode titles, public requests without forwarding reader cookies, empty/outage states, query redirects, Archive pagination/filter preservation, Search, About, existing owner sign-in, and existing article/comment fallbacks. Container smoke checks cover the new routes against Go's empty public dataset.

Browser verification uses disposable local API fixtures; it does not read or alter either cloud server's articles or owner account. Review captures are QA data, not the owner's actual writing. See [desktop Home](previews/home-react-desktop.png), [mobile Home](previews/home-react-mobile.png), and [article list](previews/home-react-articles.png).

Validation on Node 24.22.2: TypeScript, frontend lint, production build, and all 13 HTTP smoke tests passed. Chromium checked SSR/hydration, Home/Archive/Search/About navigation, teardown on client navigation, live reduced-motion changes, keyboard article visibility, mobile menu close/Escape, 360/768/1440 layouts, 200% headline bounds, one/empty/unavailable states, existing owner sign-in, and JavaScript-disabled navigation. These checks reported no page errors. Browser tooling and its Chinese test font were temporary; no browser/font dependency was added to the production application.

Exact shader reproduction, final production font selection, the full artistic treatment of Archive/Article/About, cross-browser accessibility acceptance, and agreed device/network performance budgets remain separate design/release work. This change does not add reader accounts, project-specific content types, or an independent portfolio CMS.
