/* Fixture-only local Friends study. No API calls or real external links. */
(() => {
  const root = document.documentElement;
  const grid = document.querySelector('#friends-grid');
  const mode = document.querySelector('#preview-state');
  const dialog = document.querySelector('#friend-preview');
  const reduce = document.querySelector('#reduce-motion');
  const theme = document.querySelector('#theme-toggle');
  const preference = matchMedia('(prefers-reduced-motion: reduce)');
  const art = document.querySelector('.friends-art');
  const menu = document.querySelector('.site-mobile-nav');
  document.querySelectorAll('.friend-card .friend-avatar img').forEach(img => {
    const fallback = () => { img.hidden = true; };
    img.addEventListener('error', fallback);
    if (img.complete && !img.naturalWidth) fallback();
  });
  let override = null;
  let invoker = null;
  function applyMotion() {
    const motion = override ?? !preference.matches;
    root.dataset.motion = motion ? 'on' : 'off';
    reduce.checked = !motion;
    document.querySelector('#motion-status').textContent = motion ? 'Motion on' : 'Motion off';
  }
  function setTheme(value) {
    root.dataset.theme = value;
    const dark = value === 'dark';
    theme.setAttribute('aria-label', `Switch to ${dark ? 'light' : 'dark'} theme`);
    theme.querySelector('span').textContent = dark ? 'Light' : 'Dark';
    theme.querySelector('svg').innerHTML = dark ? '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>' : '<path d="M20.5 14A9 9 0 0 1 10 3.5 9 9 0 1 0 20.5 14Z"/>';
  }
  try { if (localStorage.getItem('isaber-friends-preview-theme') === 'dark') setTheme('dark'); } catch { /* Light remains usable without storage. */ }
  theme.disabled = false;
  theme.addEventListener('click', () => {
    setTheme(root.dataset.theme === 'dark' ? 'light' : 'dark');
    try { localStorage.setItem('isaber-friends-preview-theme', root.dataset.theme); } catch { /* The page still changes theme. */ }
  });
  reduce.disabled = false;
  reduce.addEventListener('change', () => { override = !reduce.checked; applyMotion(); });
  preference.addEventListener('change', applyMotion);
  applyMotion();
  function readState() {
    const selected = new URLSearchParams(location.search).get('preview');
    mode.value = selected === 'empty' || selected === 'error' ? selected : 'sample';
    grid.hidden = mode.value !== 'sample';
    document.querySelector('#friends-empty').hidden = mode.value !== 'empty';
    document.querySelector('#friends-error').hidden = mode.value !== 'error';
    document.querySelector('#friends-count').textContent = mode.value === 'sample' ? '06 sample places to explore' : mode.value === 'empty' ? '0 places, for now' : 'Directory unavailable';
  }
  function updateState(value) {
    const query = new URLSearchParams(location.search);
    if (value === 'sample') query.delete('preview'); else query.set('preview', value);
    history.pushState(null, '', `${location.pathname}${query.size ? `?${query}` : ''}${location.hash}`);
    readState();
  }
  mode.disabled = false;
  mode.addEventListener('change', () => updateState(mode.value));
  document.querySelector('#friends-retry').addEventListener('click', () => { updateState('sample'); mode.focus({ preventScroll: true }); });
  window.addEventListener('popstate', readState);
  readState();
  document.querySelectorAll('.friend-visit').forEach(button => {
    button.disabled = false;
    button.addEventListener('click', () => {
      invoker = button;
      document.querySelector('#friend-preview-title').textContent = button.dataset.name;
      document.querySelector('#friend-preview-title').lang = /[\u3400-\u9fff]/.test(button.dataset.name) ? 'zh-CN' : 'en';
      document.querySelector('#friend-preview-site').textContent = button.dataset.site;
      dialog.showModal();
    });
  });
  dialog.addEventListener('close', () => invoker?.focus({ preventScroll: true }));
  const requestForm = document.querySelector('#friend-request-form');
  const requestDialog = document.querySelector('#request-preview');
  const requestButton = document.querySelector('#preview-request');
  const nameField = requestForm.elements.namedItem('site_name');
  const urlField = requestForm.elements.namedItem('site_url');
  const descriptionField = requestForm.elements.namedItem('description');
  const iconField = requestForm.elements.namedItem('icon_url');
  const iconImage = document.querySelector('#request-icon-image');
  const reviewIcon = document.querySelector('#request-review-icon');
  const iconStatus = document.querySelector('#request-icon-status');
  const removeIcon = document.querySelector('#remove-request-icon');
  let iconProbe = null;
  let iconTimer = null;
  let iconRequestedURL = '';
  function updateInitials() {
    const initials = [...nameField.value.trim()].slice(0, 2).join('').toUpperCase() || '?';
    document.querySelector('#request-icon-initials').textContent = initials;
    document.querySelector('#request-review-initials').textContent = initials;
  }
  function clearIconPreview() {
    if (iconProbe) {
      iconProbe.onload = iconProbe.onerror = null;
      iconProbe.removeAttribute('src');
      iconProbe = null;
    }
    clearTimeout(iconTimer);
    iconRequestedURL = '';
    for (const img of [iconImage, reviewIcon]) { img.hidden = true; img.removeAttribute('src'); }
    document.querySelector('#request-icon-initials').hidden = false;
    document.querySelector('#request-review-initials').hidden = false;
    removeIcon.hidden = !iconField.value;
  }
  function readIconURL() {
    const value = iconField.value.trim();
    let url = null;
    try { if (value) url = new URL(value); } catch { /* Validation below explains malformed URLs. */ }
    iconField.setCustomValidity(!value || (url && url.protocol === 'https:' && !url.username && !url.password) ? '' : 'Use a direct HTTPS image URL without a username or password.');
    return iconField.validity.valid ? url : null;
  }
  function loadIconPreview() {
    clearIconPreview();
    const url = readIconURL();
    if (!url) {
      iconStatus.textContent = iconField.value.trim() ? 'Use a direct HTTPS image URL.' : 'Initials will be used.';
      return;
    }
    iconRequestedURL = url.href;
    iconStatus.textContent = 'Loading icon preview…';
    const probe = new Image();
    iconProbe = probe;
    probe.referrerPolicy = 'no-referrer';
    function unavailable() {
      if (iconProbe !== probe) return;
      clearTimeout(iconTimer);
      probe.onload = probe.onerror = null;
      probe.removeAttribute('src');
      iconProbe = null;
      iconStatus.textContent = 'Icon unavailable. Initials will be used; the URL is kept.';
    }
    probe.onload = () => {
      if (iconProbe !== probe) return;
      clearTimeout(iconTimer);
      probe.onload = probe.onerror = null;
      iconProbe = null;
      for (const img of [iconImage, reviewIcon]) { img.src = url.href; img.hidden = false; }
      document.querySelector('#request-icon-initials').hidden = true;
      document.querySelector('#request-review-initials').hidden = true;
      iconStatus.textContent = 'Icon ready. Preview only; not saved.';
    };
    probe.onerror = unavailable;
    iconTimer = setTimeout(unavailable, 8000);
    probe.src = url.href;
  }
  iconField.disabled = false;
  nameField.addEventListener('input', updateInitials);
  iconField.addEventListener('input', () => {
    clearIconPreview();
    iconField.setCustomValidity('');
    iconStatus.textContent = iconField.value.trim() ? 'Preview loads when you finish editing.' : 'Initials will be used.';
  });
  iconField.addEventListener('change', loadIconPreview);
  removeIcon.addEventListener('click', () => {
    iconField.value = '';
    iconField.setCustomValidity('');
    clearIconPreview();
    iconStatus.textContent = 'Initials will be used.';
    iconField.focus({ preventScroll: true });
  });
  requestButton.disabled = false;
  for (const field of [nameField, urlField, descriptionField]) field.addEventListener('input', () => field.setCustomValidity(''));
  requestForm.addEventListener('submit', event => {
    event.preventDefault();
    nameField.setCustomValidity(nameField.value.trim() ? '' : 'Enter your website name.');
    descriptionField.setCustomValidity(descriptionField.value.trim() ? '' : 'Add a short introduction.');
    let site = null;
    try { site = new URL(urlField.value.trim()); } catch { /* Native validation explains malformed URLs. */ }
    urlField.setCustomValidity(site && site.protocol === 'https:' && !site.username && !site.password ? '' : 'Use an HTTPS website URL without a username or password.');
    const icon = readIconURL();
    if (!requestForm.reportValidity()) return;
    if (icon && icon.href !== iconRequestedURL) loadIconPreview();
    document.querySelector('#request-review-icon-url').textContent = icon?.href || '';
    document.querySelector('#request-review-icon-url').hidden = !icon;
    document.querySelector('#request-review-icon-url-label').hidden = !icon;
    document.querySelector('#request-review-name').textContent = nameField.value.trim();
    document.querySelector('#request-review-url').textContent = site.href;
    document.querySelector('#request-review-description').textContent = descriptionField.value.trim();
    const email = requestForm.elements.namedItem('email').value.trim();
    document.querySelector('#request-review-email').textContent = email;
    document.querySelector('#request-review-email').hidden = !email;
    document.querySelector('#request-review-email-label').hidden = !email;
    requestDialog.showModal();
  });
  requestDialog.addEventListener('close', () => requestButton.focus({ preventScroll: true }));
  const copyButton = document.querySelector('#copy-site-link');
  copyButton.disabled = false;
  copyButton.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText('https://isaber.top');
      document.querySelector('#copy-site-status').textContent = 'Site link copied.';
    } catch {
      document.querySelector('#copy-site-status').textContent = 'Copy unavailable. Select and copy the URL above.';
      document.querySelector('#isaber-site-url').focus({ preventScroll: true });
    }
  });
  menu.addEventListener('keydown', event => {
    if (event.key === 'Escape' && menu.open) { menu.open = false; menu.querySelector('summary').focus(); }
  });
  const circles = [...art.querySelectorAll('.friends-circle')];
  const rig = art.querySelector('.friends-art-rig');
  const mouseHover = matchMedia('(hover: hover) and (pointer: fine)');
  let hoverTweens = [];
  let hoverBase = [];
  let hoverVersion = 0;
  let hovering = false;
  function resetHover() {
    ++hoverVersion;
    hoverTweens.forEach(animation => animation.cancel());
    hoverTweens = [];
    hoverBase = [];
    hovering = false;
    delete art.dataset.attracted;
  }
  function attract(together) {
    if (root.dataset.motion !== 'on' || !mouseHover.matches || document.hidden || art.dataset.visible === 'false') return;
    if (together === hovering) return;
    hovering = together;
    // Freeze breathing during attraction, then return to the idle position
    // before breathing resumes. Translation and scale animate independently.
    const current = circles.map(circle => getComputedStyle(circle).transform);
    if (!hoverBase.length) hoverBase = current;
    const version = ++hoverVersion;
    hoverTweens.forEach(animation => animation.cancel());
    art.dataset.attracted = 'true';
    hoverTweens = circles.map((circle, index) => circle.animate([
      { transform: current[index] },
      { transform: together ? `translateX(${getComputedStyle(circle).getPropertyValue('--friends-meet-shift').trim()})` : hoverBase[index] }
    ], { duration: together ? 650 : 750, easing: 'cubic-bezier(.22,.61,.36,1)', fill: 'forwards' }));
    Promise.all(hoverTweens.map(animation => animation.finished)).then(() => {
      if (version === hoverVersion && !together) resetHover();
    }).catch(() => { /* A newer pointer movement supersedes the transition. */ });
  }
  circles.forEach(circle => circle.addEventListener('pointerenter', event => {
    if (event.pointerType === 'mouse') attract(true);
  }));
  // Use the stable artwork bounds for exit so a circle moving away from the
  // pointer cannot repeatedly trigger an enter/leave cycle.
  rig.addEventListener('pointerleave', event => { if (event.pointerType === 'mouse') attract(false); });
  rig.addEventListener('pointercancel', resetHover);
  reduce.addEventListener('change', resetHover);
  preference.addEventListener('change', resetHover);
  mouseHover.addEventListener('change', resetHover);
  window.addEventListener('blur', resetHover);
  const visibility = () => { root.dataset.documentHidden = String(document.hidden); if (document.hidden) resetHover(); };
  visibility(); document.addEventListener('visibilitychange', visibility);
  if ('IntersectionObserver' in window) new IntersectionObserver(entries => {
    art.dataset.visible = String(entries[0].isIntersecting);
    if (!entries[0].isIntersecting) resetHover();
  }).observe(art);
})();
