# Friends page design review

Status: The owner approved the Feature 6 proposal. Direction 04 uses two circles breathing at fixed positions, meeting only on hover, and an optional profile icon URL in the friend-link request. The owner selected the current card layout on 2026-11-18: large circular icons vertically centered on the right, name and description on the left, and domain/visit action along the bottom. Preserve this version, including the owner’s circle colors. The owner’s latest local refinements also set the card header minimum height to 24px, the icon position to 42%, and the introduction width to 60ch; these supersede the earlier spacing values. React/API integration is now implemented locally with migration 10; see [Friends operations](../operations/friends.md). The standalone prototype remains a visual reference. Statistics (Feature 5) remain deferred. All implementation remains local; no commit, push or deployment has been performed.

## Composition

Keep the black horizontal navigation and add Friends between Archive and About. The local prototype demonstrates that navigation without editing the production header. “Good company.” sets a large, two-line sans-serif headline on a warm coral/rose field. Two filled geometric circles preserve the owner’s latest colors and positions. Each breathes gently in size with a staggered period; their centers stay fixed. Hovering either circle with a mouse smoothly brings both into partial overlap and holds them there. Leaving the stable artwork bounds returns them to their idle positions before breathing resumes. Touch keeps stationary breathing. Reduced motion disables both breathing and attraction.

The directory uses the site's selected #FFE2ED light background, with a two-column, square-edged grid of spacious entries. Violet, lilac, and peach blocks create a flat color rhythm. Each entry shows a sequence number, avatar or initials, name, brief description, domain, and a clear visit action. Mobile stacks the entries in one column. A “Let’s connect” section follows the directory. Its left column offers the real iSaber URL, the owner’s supplied 256px JPG profile image and short introduction, plus a copy-link action. Its right column previews a request form: required website name, HTTPS URL and introduction, plus an optional HTTPS profile icon URL and private reply email. A square 256 × 256 image is suggested. Finishing an icon URL edit loads a browser-side image preview without sending a referrer. The form and confirmation show a circular crop; unavailable images fall back to initials while retaining the URL. Removing the URL restores initials. No images are uploaded or requests saved. The hero links directly to this section. Inputs stay only in the current page’s memory; a labeled native dialog shows the request preview without sending, storing, or publishing it. Dark mode uses the existing reading palette while preserving the warm header and violet accents.

## Review scope

Run `python3 docs/design/serve.py --port 8765`, then open <http://127.0.0.1:8765/friends-prototype.html>.

All six sample names, introductions, domains, and monogram avatars and two locally drawn sample icons are fictional. Their domains use reserved `.example` names. The review bar identifies the sample content, and Visit site opens a labeled native sample dialog instead of attempting to open a nonexistent website. Real external links will open in a new tab with `noopener noreferrer` during integration. Directory fixtures use local icons. Only a valid HTTPS image URL entered in the request form triggers a remote image preview; no favicon discovery or server fetch occurs. Without JavaScript, entries remain readable and sample actions are disabled.

The review controls demonstrate sample, empty, and unavailable collections. Their state survives reload/back/forward through a preview-only URL parameter; Retry returns to the fixture collection. The theme uses a dedicated local-storage key and does not change the blog's cookie. Motion follows the system initially and allows an explicit opt-in for this design study. Reduced motion disables geometric animations and arrow transitions; off-screen and background animation pauses. Keyboard users can open/close dialogs and the mobile menu, with focus restored.

## Planned integration after visual approval

- `/friends`: public list of published entries in owner-selected order.
- Owner Admin: create/edit name, HTTPS URL, short introduction and optional profile icon URL; reorder, publish, hide.
- Add guest link requests with owner moderation: pending requests remain private until approved; approval should produce a separately reviewable directory entry. Confirm reply-email handling and rate limits during API design.
- Keep draft/hidden entries and their metadata owner-only. Validate website and profile icon URLs. Follow existing authentication, CSRF, version-conflict, and no-store conventions.
- Use an additive, reviewed migration and meaningful API/privacy/order tests. Do not simulate these management operations in the visual prototype.

Review the header, filled-circle motion, request section, card density, color rhythm, bilingual text, empty/unavailable states, light/dark modes, and mobile layout before implementing the service.

## Verification

Chromium checks passed at 320, 360, 390, 620, 768, 1440, and 1900 CSS pixels, including a long unbroken site name/domain and a 200% root font. Keyboard checks cover the sample dialog, Escape/focus restoration, and mobile menu dismissal. Preview states survive reload/back/forward; fixture retry restores the collection. Reduced motion follows the system with explicit review opt-in, and off-screen artwork pauses. Theme persistence retains the black navigation. The no-JavaScript fallback shows all six entries with unavailable sample actions disabled. No external requests, unexpected asset failures, or page errors occurred.

Automated WCAG A/AA checks reported no violations for light mobile, light desktop, dark desktop, the sample dialog, empty collection, and unavailable collection. These automated checks supplement visual/keyboard review; final owner acceptance remains pending.

Saved captures:

- [Warm filled-circle header](previews/friends-header-desktop.png)
- [Desktop page](previews/friends-desktop.png)
- [Mobile page](previews/friends-mobile.png)
- [Dark desktop page](previews/friends-dark-desktop.png)

Direction 02 verification: filled-circle rendering, request-section navigation, clipboard success, required/whitespace validation, HTTPS URL validation (including rejection of embedded credentials), safe text-only preview, optional private reply email, Escape/focus restoration, preservation while editing, and empty inputs on reload passed. The request UI was checked at 320, 390, 768, 1440 and 1900 pixels. Accessibility checks also passed for the request preview and request section in light, dark and mobile states. No requests were submitted; without JavaScript, preview/copy actions remain disabled.

Additional captures:

- [Request section, desktop](previews/friends-request-desktop.png)
- [Request section, mobile](previews/friends-request-mobile.png)

Direction 03 verification: Chromium confirmed exactly two circles, a separated start/end, partial overlap in the middle, and constant circle size. Reduced motion and off-screen pausing work. Optional icon selection and removal, invalid MIME/size/image rejection, circular preview in the form and confirmation, initials fallback, broken directory image fallback, and no upload/external traffic passed. Existing request validation, keyboard focus, light/dark accessibility and responsive checks also passed. Two local geometric SVG samples demonstrate icons; other entries demonstrate initials.

Hover verification: Chromium checks passed for either-circle entry, continuity of enter/exit keyframes, held overlap, automatic-loop resumption, rapid exit/re-entry, reduced-motion cancellation, off-screen pause/reset, touch fallback and absence of page errors.

Local integration verification: the React page and owner workspace use real Go endpoints. Browser checks cover private request submission, approval as a hidden draft, explicit publication, ordering, hiding, and exclusion of reply email from public HTML. Captures: [React desktop](previews/friends-react-desktop.png), [React mobile](previews/friends-react-mobile.png), [Admin directory](previews/friends-admin-desktop.png). These captures use isolated synthetic data, never production content.
