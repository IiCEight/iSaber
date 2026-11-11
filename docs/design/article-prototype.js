const contents = document.querySelector('#contents');
const smallScreen = matchMedia('(max-width: 1000px)');
const setContents = () => { contents.open = !smallScreen.matches; };
setContents();
smallScreen.addEventListener('change', setContents);
const tocLinks = [...contents.querySelectorAll('a')];
const sections = tocLinks.map(link => document.querySelector(link.getAttribute('href')));
if ('IntersectionObserver' in window) {
  const visible = new Set();
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (entry.isIntersecting) visible.add(entry.target.id);
      else visible.delete(entry.target.id);
    }
    const current = sections.find(section => visible.has(section.id));
    for (const link of tocLinks) {
      if (current && link.hash === '#' + current.id) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    }
  }, { rootMargin: '-5% 0px -55% 0px' });
  for (const section of sections) observer.observe(section);
}
const copy = document.querySelector('#copy-code');
copy.hidden = false;
copy.addEventListener('click', async () => {
  const status = document.querySelector('#copy-status');
  try {
    await navigator.clipboard.writeText(document.querySelector('#code-sample').textContent);
    status.textContent = 'Code copied.';
  } catch {
    status.textContent = 'Copy is unavailable here. Select the code to copy it manually.';
  }
});
const state = document.querySelector('#comments-state');
const feedback = document.querySelector('#comments-feedback');
const form = document.querySelector('#comment-form');
const submit = document.querySelector('#comment-submit');
function renderComments() {
  const disabled = state.value === 'disabled' || state.value === 'unavailable';
  document.querySelector('#comment-list').hidden = state.value === 'empty' || state.value === 'unavailable';
  document.querySelector('#comments-empty').hidden = state.value !== 'empty';
  document.querySelector('#comments-disabled').hidden = state.value !== 'disabled';
  document.querySelector('#retry-comments').hidden = state.value !== 'unavailable';
  feedback.hidden = state.value !== 'unavailable';
  feedback.textContent = state.value === 'unavailable' ? 'Comments are temporarily unavailable. The article remains readable; your entered text is kept.' : '';
  for (const control of form.querySelectorAll('input,textarea,button')) control.disabled = disabled;
}
state.addEventListener('change', renderComments);
document.querySelector('#retry-comments').addEventListener('click', () => { state.value = 'approved'; renderComments(); });
form.addEventListener('submit', event => {
  event.preventDefault();
  if (!form.display_name.value.trim() || !form.body.value.trim()) {
    feedback.textContent = 'Enter a nonblank display name and comment.';
  } else {
    feedback.textContent = 'Preview only: this would be sent for owner approval. Nothing was sent or published; your text is kept for review.';
  }
  feedback.hidden = false;
  feedback.scrollIntoView({ block: 'nearest' });
});
renderComments();
