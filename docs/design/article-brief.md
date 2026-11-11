# Article design review

Status: First local prototype, pending owner review. The production Article route is unchanged. No push or CI/CD is part of this iteration.

## Reading experience

An expressive coral/pink opening connects the article to Home and Archive. A large title, short summary, category/tags, language, and separate written/published/updated dates establish context. A cropped composition of circular lenses supports the title without requiring a cover image, which the current article model does not provide.

The reading area uses Archive's indigo palette, a narrow text measure, generous line height, and clear heading levels. Desktop has a small sticky contents rail; mobile has an ordinary collapsible contents block above the text. Links jump directly to sections. Essential content never waits for animation. Circle breathing is confined to the opening and respects reduced motion.

The prototype includes explicitly sample English/Chinese content, links, lists, a quote, a locally drawn SVG figure, highlighted code, a table, and native MathML. They test typography and overflow; they are not the owner's published writing. Code and tables use local horizontal scrolling on narrow screens, with labeled keyboard-focusable scroll regions. Code can be copied with honest success/failure feedback. The external image/loading/privacy rules of the existing Markdown renderer remain requirements for integration.

Keep the primary task—reading—in front. No floating toolbar, modal reading controls, pinned hero, or invented reading-time metric. Footer actions lead back to Archive or the top. Existing horizontal navigation remains familiar.

## Comments and states

Preserve the existing guest comment flow: display name and plain-text comment, no email or account, visible moderation notice. The prototype's state selector covers approved samples, no approved comments, new comments disabled, and an unavailable comments service. Comment submission is a browser-only simulation with an explicit preview message and no network request. It never pretends to publish or contact the owner. Approved samples remain visible when new comments are disabled. Comments never block reading the article.

## Review and integration boundaries

Review title composition, body color/measure, contents placement, code/table/math treatment, metadata, and mobile reading before integration. Long titles, Chinese paragraphs, narrow screens, enlarged text, keyboard use, reduced motion, and disabled JavaScript must remain readable.

After approval, adapt the existing server Article route, Markdown renderer, and comments component. Keep public/private article and asset authorization, unavailable comments behavior, three source dates, raw HTML restrictions, external-image safeguards, moderation, rate limits, and retry/idempotency handling. Generated contents links and copy buttons are presentation enhancements; they must not change stored Markdown or flatten accessible code/math. No backend or account-system change is proposed.

Preview: `http://127.0.0.1:8765/article-prototype.html`.

## Local verification

Chromium checks passed for heading/date structure, contents links, image loading, native math, actual clipboard copy, all four comment states, preserved entered text, and absence of POST requests. Layouts at 1440/768/390/360 pixels, keyboard-focusable overflow regions, 200% base text, reduced motion, and JavaScript-disabled reading were checked. Desktop/mobile captures were visually reviewed. The existing frontend routes remain unchanged; this prototype is served independently on port 8765.
