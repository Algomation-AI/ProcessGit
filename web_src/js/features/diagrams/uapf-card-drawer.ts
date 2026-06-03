// UAPF v2.5.0 chapter 13.16: side-panel drawer that opens the Algorithm
// Card viewer over the BPMN diagram when the user clicks the algorithm
// task overlay.
//
// The drawer:
//   - slides in from the right
//   - covers roughly 50% of the viewport width on desktop, full width on mobile
//   - shows the same polymorphic card viewer used in the Preview tab (lazy-
//     imports ./card.ts so we don't pull js-yaml into the BPMN bundle unless
//     the drawer is actually opened)
//   - has a close button and dismisses on backdrop click / Escape key
//   - preserves the BPMN process context behind it (no navigation away)

let drawerEl: HTMLDivElement | null = null;
let backdropEl: HTMLDivElement | null = null;
let escHandler: ((e: KeyboardEvent) => void) | null = null;
let currentCardRef: string | null = null;

function ensureDom(): void {
  if (drawerEl && backdropEl) return;

  backdropEl = document.createElement('div');
  backdropEl.className = 'uapf-card-drawer-backdrop';
  backdropEl.style.cssText = [
    'position: fixed',
    'inset: 0',
    'background: rgba(0, 0, 0, 0.32)',
    'opacity: 0',
    'pointer-events: none',
    'transition: opacity 200ms ease',
    'z-index: 998',
  ].join('; ');
  backdropEl.addEventListener('click', closeCardDrawer);

  drawerEl = document.createElement('div');
  drawerEl.className = 'uapf-card-drawer';
  drawerEl.style.cssText = [
    'position: fixed',
    'top: 0',
    'right: 0',
    'width: min(720px, 100vw)',
    'height: 100vh',
    'background: #FFFFFF',
    'box-shadow: -8px 0 24px rgba(0, 0, 0, 0.16)',
    'transform: translateX(100%)',
    'transition: transform 280ms cubic-bezier(0.16, 1, 0.3, 1)',
    'z-index: 999',
    'display: flex',
    'flex-direction: column',
    'overflow: hidden',
  ].join('; ');

  // Header bar with title + close button
  const header = document.createElement('div');
  header.style.cssText = [
    'display: flex',
    'align-items: center',
    'justify-content: space-between',
    'padding: 12px 20px',
    'background: #F8F7F4',
    'border-bottom: 1px solid #E5E2DC',
    'flex-shrink: 0',
  ].join('; ');
  const title = document.createElement('div');
  title.id = 'uapf-card-drawer-title';
  title.style.cssText = 'font-size: 13px; color: #5F5E5A; font-family: "Source Code Pro", monospace;';
  title.textContent = 'Algorithm Card';
  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.setAttribute('aria-label', 'Close');
  closeBtn.style.cssText = [
    'border: 1px solid #E5E2DC',
    'background: #FFFFFF',
    'color: #5F5E5A',
    'border-radius: 4px',
    'padding: 4px 10px',
    'font-size: 13px',
    'cursor: pointer',
  ].join('; ');
  closeBtn.textContent = '✕ Close';
  closeBtn.addEventListener('click', closeCardDrawer);
  header.appendChild(title);
  header.appendChild(closeBtn);

  // Body (scrollable)
  const body = document.createElement('div');
  body.id = 'uapf-card-drawer-body';
  body.style.cssText = [
    'flex: 1',
    'overflow-y: auto',
    'background: #FFFFFF',
  ].join('; ');

  drawerEl.appendChild(header);
  drawerEl.appendChild(body);

  document.body.appendChild(backdropEl);
  document.body.appendChild(drawerEl);
}

function getRepoFromUrl(): {owner: string; repo: string; branch: string} | null {
  // URL shape: /{owner}/{repo}/src/branch/{branch}/{path...}
  const m = window.location.pathname.match(/^\/([^\/]+)\/([^\/]+)\/src\/(?:branch|commit|tag)\/([^\/]+)\//);
  if (!m) return null;
  return {owner: m[1], repo: m[2], branch: m[3]};
}

function buildRawCardUrl(cardRef: string): string | null {
  const ctx = getRepoFromUrl();
  if (!ctx) return null;
  // Card IDs follow algo.<package>.<name>. The file in the repo lives at
  // algorithms/<name>.card.yaml — strip everything before the final dot.
  const parts = cardRef.split('.');
  const fileBase = parts[parts.length - 1];
  return `/${encodeURIComponent(ctx.owner)}/${encodeURIComponent(ctx.repo)}/raw/branch/${encodeURIComponent(ctx.branch)}/algorithms/${encodeURIComponent(fileBase)}.card.yaml`;
}

export async function openCardDrawer(cardRef: string): Promise<void> {
  ensureDom();
  if (!drawerEl || !backdropEl) return;

  currentCardRef = cardRef;

  // Update title
  const title = drawerEl.querySelector('#uapf-card-drawer-title') as HTMLDivElement | null;
  if (title) title.textContent = cardRef;

  // Reset body and show loading
  const body = drawerEl.querySelector('#uapf-card-drawer-body') as HTMLDivElement | null;
  if (body) {
    body.innerHTML = '<div style="padding:24px;color:#888780;font-size:13px;">Loading card…</div>';
  }

  // Open drawer
  requestAnimationFrame(() => {
    if (!drawerEl || !backdropEl) return;
    backdropEl.style.opacity = '1';
    backdropEl.style.pointerEvents = 'auto';
    drawerEl.style.transform = 'translateX(0)';
  });

  // Escape key handler
  if (!escHandler) {
    escHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeCardDrawer();
    };
    document.addEventListener('keydown', escHandler);
  }

  // Fetch raw card YAML
  const rawUrl = buildRawCardUrl(cardRef);
  if (!rawUrl) {
    if (body) body.innerHTML = '<div style="padding:24px;color:#E24B4A;">Could not resolve card URL from page context.</div>';
    return;
  }

  let yamlText: string;
  try {
    const r = await fetch(rawUrl, {headers: {Accept: 'text/plain', 'X-Requested-With': 'XMLHttpRequest'}});
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    yamlText = await r.text();
  } catch (err) {
    if (body) body.innerHTML = `<div style="padding:24px;color:#E24B4A;">Failed to load card: ${(err as Error).message}</div>`;
    return;
  }

  // Guard against race: user may have closed/reopened with a different card
  if (currentCardRef !== cardRef || !body) return;

  // Lazy-load the card adapter (same module used by the Preview tab) and render
  try {
    const {createCardAdapter} = await import('./card.ts');
    body.innerHTML = '';
    const adapter = createCardAdapter(body, null);
    await adapter.renderPreview(yamlText);
  } catch (err) {
    body.innerHTML = `<div style="padding:24px;color:#E24B4A;">Failed to mount viewer: ${(err as Error).message}</div>`;
  }
}

export function closeCardDrawer(): void {
  if (!drawerEl || !backdropEl) return;
  drawerEl.style.transform = 'translateX(100%)';
  backdropEl.style.opacity = '0';
  backdropEl.style.pointerEvents = 'none';
  currentCardRef = null;
  if (escHandler) {
    document.removeEventListener('keydown', escHandler);
    escHandler = null;
  }
}
