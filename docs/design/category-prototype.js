/* Local, fixture-only interaction study. No accounts, API calls, or writes. */
(() => {
  const root = document.documentElement;
  const list = document.querySelector('#category-articles');
  const form = document.querySelector('#category-search-form');
  const panel = document.querySelector('#category-search-panel');
  const opener = document.querySelector('#open-search');
  const chips = document.querySelector('#condition-chips');
  const mode = document.querySelector('#preview-state');
  const dialog = document.querySelector('#sample-preview');
  const reduce = document.querySelector('#reduce-motion');
  const preference = matchMedia('(prefers-reduced-motion: reduce)');
  const fields = ['q', 'tag', 'language'];
  const orders = ['written', 'written_oldest', 'updated', 'updated_oldest', 'newest', 'oldest', 'relevance'];
  const size = 10;
  const items = [...list.children].map(element => ({
    element, id: element.id, title: element.querySelector('h3').textContent,
    summary: element.querySelector('.archive-summary').textContent,
    language: element.dataset.language, tags: element.dataset.tags.split(' '),
    pinned: element.dataset.categoryPin === 'true',
    written: element.dataset.written, updated: element.dataset.updated, published: element.dataset.published,
  }));
  let applied;
  let pageNumber;
  let sort;
  let motionOverride = null;
  let motion = false;
  const rowAnimations = new Map();
  const dateFormat = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Shanghai', day: '2-digit', month: 'short', year: 'numeric' });

  function readURL() {
    const query = new URLSearchParams(location.search);
    applied = Object.fromEntries(fields.map(key => [key, (query.get(key) ?? '').trim().slice(0, 200)]));
    sort = orders.includes(query.get('sort')) ? query.get('sort') : 'written';
    if (!applied.q && sort === 'relevance') sort = 'written';
    const rawPage = Number(query.get('page') ?? 1);
    pageNumber = Number.isSafeInteger(rawPage) && rawPage > 0 ? rawPage : 1;
    mode.value = ['empty', 'error'].includes(query.get('preview')) ? query.get('preview') : 'sample';
  }
  function writeURL() {
    const query = new URLSearchParams();
    for (const key of fields) if (applied[key]) query.set(key, applied[key]);
    if (sort !== 'written') query.set('sort', sort);
    if (pageNumber > 1) query.set('page', String(pageNumber));
    if (mode.value !== 'sample') query.set('preview', mode.value);
    const target = `${location.pathname}${query.size ? `?${query}` : ''}#category-writing`;
    if (`${location.pathname}${location.search}${location.hash}` !== target) history.pushState(null, '', target);
  }
  function syncForm() {
    for (const key of fields) {
      const field = form.elements.namedItem(key);
      if (field.tagName === 'SELECT') {
        field.querySelectorAll('[data-unknown]').forEach(option => option.remove());
        if (applied[key] && ![...field.options].some(option => option.value === applied[key])) {
          const option = new Option(`Selected: ${applied[key]}`, applied[key]);
          option.dataset.unknown = 'true'; field.add(option);
        }
      }
      field.value = applied[key];
    }
  }
  function setOpen(open, focus = true) {
    if (open) syncForm();
    panel.classList.toggle('is-collapsed', !open);
    panel.inert = !open; panel.setAttribute('aria-hidden', String(!open));
    opener.setAttribute('aria-expanded', String(open));
    if (focus) requestAnimationFrame(() => (open ? form.q : opener).focus({ preventScroll: true }));
  }
  function termMatches(value, term) {
    if (/^[a-z0-9_]+$/.test(term)) return new RegExp(`(^|[^a-z0-9_])${term}($|[^a-z0-9_])`).test(value);
    return value.includes(term);
  }
  function rank(item) {
    const title = item.title.toLocaleLowerCase();
    const metadata = `${item.summary} ${item.tags.join(' ')}`.toLocaleLowerCase();
    let result = 0;
    for (const term of applied.q.toLocaleLowerCase().split(/\s+/).filter(Boolean)) {
      if (termMatches(title, term)) continue;
      if (!termMatches(metadata, term)) return -1;
      result = 1;
    }
    return result;
  }
  function sortField() {
    return sort.startsWith('written') ? 'written' : sort.startsWith('updated') ? 'updated' : sort === 'relevance' ? 'relevance' : 'newest';
  }
  function dateValue(item) {
    return sortField() === 'written' ? item.written : sortField() === 'updated' ? item.updated : item.published;
  }
  function comparator(a, b) {
    if (!applied.q && a.pinned !== b.pinned) return a.pinned ? -1 : 1;
    if (sort === 'relevance') {
      const difference = rank(a) - rank(b);
      if (difference) return difference;
    }
    const oldest = sort.endsWith('oldest');
    const left = dateValue(a), right = dateValue(b);
    if (left !== right) return (left < right ? -1 : 1) * (oldest ? 1 : -1);
    return a.id.localeCompare(b.id) * (oldest ? 1 : -1);
  }
  function labelFor(key) {
    if (key === 'language') return applied[key] === 'zh' ? '中文' : applied[key] === 'en' ? 'English' : applied[key];
    if (key === 'tag') return [...form.tag.options].find(option => option.value === applied[key])?.textContent ?? applied[key];
    return applied[key];
  }
  function renderChips() {
    const active = fields.filter(key => applied[key]);
    document.querySelector('#active-conditions').hidden = !active.length;
    document.querySelector('.archive-filter-dot').hidden = !active.length;
    chips.replaceChildren();
    for (const key of active) {
      const button = document.createElement('button');
      const label = `${{ q: 'Search', tag: 'Tag', language: 'Language' }[key]}: ${labelFor(key)}`;
      button.type = 'button'; button.className = 'archive-condition-chip'; button.dataset.key = key;
      button.setAttribute('aria-label', `Remove ${label}`);
      const text = document.createElement('span'), cross = document.createElement('span');
      text.textContent = label; cross.textContent = '×'; cross.setAttribute('aria-hidden', 'true');
      button.append(text, cross); chips.append(button);
    }
  }
  function renderPagination(totalPages, count) {
    const navigation = document.querySelector('#category-pagination');
    const controls = document.querySelector('#pagination-controls');
    navigation.hidden = totalPages <= 1 || !count;
    controls.replaceChildren();
    function button(label, target, className, disabled = false) {
      const control = document.createElement('button');
      control.type = 'button'; control.className = className; control.disabled = disabled;
      control.textContent = label; control.dataset.page = String(target); return control;
    }
    const previous = button('←', pageNumber - 1, 'pagination-direction', pageNumber === 1);
    previous.setAttribute('aria-label', 'Previous page'); controls.append(previous);
    const pages = document.createElement('div'); pages.className = 'pagination-pages';
    for (let number = 1; number <= totalPages; number++) {
      const control = button(String(number), number, 'pagination-page');
      control.setAttribute('aria-label', `Page ${number}`);
      if (number === pageNumber) control.setAttribute('aria-current', 'page');
      pages.append(control);
    }
    controls.append(pages);
    const next = button('→', pageNumber + 1, 'pagination-direction', pageNumber === totalPages);
    next.setAttribute('aria-label', 'Next page'); controls.append(next);
  }
  function render() {
    const matching = mode.value !== 'sample' ? [] : items.filter(item =>
      (!applied.tag || item.tags.includes(applied.tag)) && (!applied.language || item.language === applied.language) && rank(item) >= 0
    ).sort(comparator);
    const totalPages = Math.max(1, Math.ceil(matching.length / size));
    pageNumber = Math.min(pageNumber, totalPages);
    const offset = (pageNumber - 1) * size;
    const current = matching.slice(offset, offset + size);
    for (const item of items) item.element.hidden = true;
    for (const [index, item] of current.entries()) {
      item.element.hidden = false;
      item.element.querySelector('.row-number').textContent = `${String(offset + index + 1).padStart(2, '0')} /`;
      const value = dateValue(item), written = sortField() === 'written';
      const label = written ? 'Written date' : sortField() === 'updated' ? 'Last updated' : 'Published date';
      const time = item.element.querySelector('time'); time.dateTime = value; time.title = label;
      time.querySelector('.archive-date-label').textContent = `${label}: `;
      time.querySelector('.row-date').textContent = dateFormat.format(new Date(written ? `${value}T00:00:00+08:00` : value));
    }
    list.replaceChildren(...matching.map(item => item.element), ...items.filter(item => !matching.includes(item)).map(item => item.element));
    const filtered = fields.some(key => applied[key]);
    document.querySelector('#result-count').textContent = mode.value === 'error' ? 'Collection unavailable' : matching.length ? `${offset + 1}–${offset + current.length} of ${matching.length} articles` : '0 articles';
    document.querySelector('#category-total').textContent = mode.value === 'error' ? 'Collection unavailable' : `${mode.value === 'empty' ? 0 : items.length} articles in this thread`;
    document.querySelector('#empty-state').hidden = mode.value === 'error' || matching.length > 0;
    document.querySelector('#error-state').hidden = mode.value !== 'error';
    document.querySelector('#empty-label').textContent = mode.value === 'empty' ? 'The first chapter' : 'A different thread';
    document.querySelector('#empty-title').textContent = mode.value === 'empty' ? 'The first chapter is on its way.' : 'No articles on this thread.';
    document.querySelector('#empty-copy').textContent = mode.value === 'empty' ? 'New writing in Technology will appear here.' : 'Try another word or clear a filter to explore more of Technology.';
    document.querySelector('#empty-reset').hidden = mode.value === 'empty' || !filtered;
    document.querySelector('#collection-end').textContent = matching.length && pageNumber === totalPages ? 'You’ve reached the end of this category.' : '';
    document.querySelector('#sort-hint').textContent = applied.q ? 'Search matches' : 'Category pins first';
    for (const button of document.querySelectorAll('[data-sort]')) {
      button.setAttribute('aria-pressed', String(button.dataset.sort === sortField()));
      button.hidden = button.dataset.sort === 'relevance' && !applied.q;
    }
    const oldest = sort.endsWith('oldest');
    document.querySelector('#direction-label').textContent = oldest ? 'Oldest first' : 'Newest first';
    document.querySelector('#sort-direction .archive-sort-arrow').textContent = oldest ? '↑' : '↓';
    document.querySelector('#sort-direction').setAttribute('aria-label', `Change to ${oldest ? 'newest' : 'oldest'} first`);
    document.querySelector('#sort-direction').hidden = sort === 'relevance';
    renderChips(); renderPagination(totalPages, matching.length);
  }
  function navigate(focus = true) { render(); writeURL(); setOpen(false, focus); }
  function clearFilters() {
    applied = { q: '', tag: '', language: '' }; pageNumber = 1;
    if (sort === 'relevance') sort = 'written';
    navigate();
  }
  form.addEventListener('submit', event => {
    event.preventDefault(); applied = Object.fromEntries(fields.map(key => [key, form.elements.namedItem(key).value.trim()]));
    pageNumber = 1; if (!applied.q && sort === 'relevance') sort = 'written';
    navigate(); opener.scrollIntoView({ block: 'nearest' });
  });
  document.querySelector('#reset-form').addEventListener('click', () => { for (const key of fields) form.elements.namedItem(key).value = ''; form.q.focus(); });
  opener.addEventListener('click', () => setOpen(opener.getAttribute('aria-expanded') !== 'true'));
  document.querySelector('#close-search').addEventListener('click', () => setOpen(false));
  panel.addEventListener('keydown', event => { if (event.key === 'Escape') { event.preventDefault(); setOpen(false); } });
  document.querySelectorAll('[data-open-search]').forEach(link => link.addEventListener('click', event => {
    event.preventDefault(); document.querySelector('.site-mobile-nav').open = false;
    setOpen(true); panel.scrollIntoView({ block: 'start' });
  }));
  const mobileMenu = document.querySelector('.site-mobile-nav');
  mobileMenu.addEventListener('keydown', event => { if (event.key === 'Escape') { mobileMenu.open = false; mobileMenu.querySelector('summary').focus(); } });
  document.querySelector('#clear-all').addEventListener('click', clearFilters);
  document.querySelector('#empty-reset').addEventListener('click', clearFilters);
  chips.addEventListener('click', event => {
    const button = event.target.closest('[data-key]'); if (!button) return;
    const index = [...chips.children].indexOf(button); applied[button.dataset.key] = ''; pageNumber = 1;
    if (!applied.q && sort === 'relevance') sort = 'written';
    navigate(false); (chips.children[Math.min(index, chips.children.length - 1)] ?? opener).focus({ preventScroll: true });
  });
  document.querySelectorAll('[data-sort]').forEach(button => button.addEventListener('click', () => {
    sort = button.dataset.sort; pageNumber = 1; navigate(false); button.focus({ preventScroll: true });
  }));
  document.querySelector('#sort-direction').addEventListener('click', event => {
    const field = sortField(); const oldest = sort.endsWith('oldest');
    sort = field === 'written' ? oldest ? 'written' : 'written_oldest' : field === 'updated' ? oldest ? 'updated' : 'updated_oldest' : oldest ? 'newest' : 'oldest';
    pageNumber = 1; navigate(false); event.currentTarget.focus({ preventScroll: true });
  });
  document.querySelector('#pagination-controls').addEventListener('click', event => {
    const button = event.target.closest('[data-page]'); if (!button || button.disabled) return;
    pageNumber = Number(button.dataset.page); navigate(false);
    document.querySelector('#category-writing').focus({ preventScroll: true });
    document.querySelector('#category-writing').scrollIntoView({ block: 'start' });
  });
  mode.addEventListener('change', () => { pageNumber = 1; navigate(false); });
  document.querySelector('#retry').addEventListener('click', () => { mode.value = 'sample'; navigate(); });
  window.addEventListener('popstate', () => { readURL(); render(); setOpen(false, false); });
  list.addEventListener('click', event => {
    const link = event.target.closest('.sample-article-link'); if (!link) return;
    event.preventDefault(); const item = items.find(item => item.element.contains(link));
    document.querySelector('#sample-title').textContent = item.title;
    document.querySelector('#sample-title').lang = item.language === 'zh' ? 'zh-CN' : 'en';
    document.querySelector('#sample-summary').textContent = item.summary;
    document.querySelector('#sample-summary').lang = item.language === 'zh' ? 'zh-CN' : 'en'; dialog.showModal();
  });

  const themeButton = document.querySelector('#theme-toggle');
  function applyTheme(theme) {
    root.dataset.theme = theme;
    const dark = theme === 'dark';
    themeButton.setAttribute('aria-label', `Switch to ${dark ? 'light' : 'dark'} theme`);
    themeButton.title = themeButton.getAttribute('aria-label'); themeButton.querySelector('span').textContent = dark ? 'Light' : 'Dark';
    themeButton.querySelector('svg').innerHTML = dark ? '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>' : '<path d="M20.5 14A9 9 0 0 1 10 3.5 9 9 0 1 0 20.5 14Z"/>';
  }
  try { const saved = localStorage.getItem('isaber-category-preview-theme'); if (saved === 'dark') applyTheme(saved); } catch { /* A static light preview is always usable. */ }
  themeButton.disabled = false;
  themeButton.addEventListener('click', () => {
    applyTheme(root.dataset.theme === 'dark' ? 'light' : 'dark');
    try { localStorage.setItem('isaber-category-preview-theme', root.dataset.theme); } catch { /* Keep this page usable without storage. */ }
  });
  function applyMotion() {
    motion = motionOverride ?? !preference.matches;
    root.dataset.motion = motion ? 'on' : 'off'; reduce.checked = !motion;
    document.querySelector('#motion-status').textContent = motion ? 'Motion on' : 'Motion off';
    if (!motion) { for (const animation of rowAnimations.values()) animation.cancel(); rowAnimations.clear(); }
  }
  reduce.disabled = false;
  reduce.addEventListener('change', () => { motionOverride = !reduce.checked; applyMotion(); });
  preference.addEventListener('change', applyMotion);
  document.addEventListener('visibilitychange', () => { root.dataset.documentHidden = String(document.hidden); });
  applyMotion();
  if ('IntersectionObserver' in window) {
    const seen = new WeakSet();
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) {
        if (!entry.isIntersecting || seen.has(entry.target)) continue;
        seen.add(entry.target);
        if (motion && entry.target.animate) {
          const animation = entry.target.animate([{ opacity: .45, transform: 'translateY(12px)' }, { opacity: 1, transform: 'translateY(0)' }], { duration: 400, easing: 'ease-out' });
          rowAnimations.set(entry.target, animation); animation.finished.then(() => rowAnimations.delete(entry.target)).catch(() => {});
        }
      }
    }, { threshold: .04 });
    for (const item of items) {
      observer.observe(item.element);
      item.element.addEventListener('focusin', () => { rowAnimations.get(item.element)?.cancel(); rowAnimations.delete(item.element); });
    }
    const art = document.querySelector('.category-art');
    new IntersectionObserver(entries => { art.dataset.visible = String(entries[0].isIntersecting); }).observe(art);
  }
  readURL(); render(); syncForm(); mode.disabled = false;
  document.querySelector('#category-sort').hidden = false; opener.hidden = false; panel.hidden = false; setOpen(false, false);
})();
