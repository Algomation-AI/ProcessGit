import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {showErrorToast, showInfoToast} from '../../modules/toast.ts';
import type {ProcessGitViewerPayload} from './types.ts';

function encodePath(path: string): string {
  return path
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

function toMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function injectBaseTag(html: string, baseHref: string): string {
  if (/<base\s/i.test(html)) {
    return html.replace(/<base\b[^>]*>/i, `<base href="${baseHref}">`);
  }

  if (/<head\b[^>]*>/i.test(html)) {
    return html.replace(/<head\b[^>]*>/i, (match) => `${match}\n<base href="${baseHref}">`);
  }

  return `<!doctype html><html><head><base href="${baseHref}"></head><body>${html}</body></html>`;
}

function parsePayload(script: HTMLScriptElement | null): ProcessGitViewerPayload | null {
  if (!script?.textContent) return null;
  try {
    const raw = JSON.parse(script.textContent) as Partial<ProcessGitViewerPayload>;
    if (!raw || typeof raw.entryRawUrl !== 'string' || typeof raw.path !== 'string' || typeof raw.apiUrl !== 'string') {
      return null;
    }
    return {
      id: raw.id ?? '',
      type: 'html',
      repoLink: raw.repoLink ?? '',
      branch: raw.branch ?? '',
      ref: raw.ref ?? '',
      path: raw.path,
      dir: raw.dir ?? '',
      lastCommit: raw.lastCommit ?? '',
      entryRawUrl: raw.entryRawUrl,
      targets: raw.targets ?? {},
      editAllow: raw.editAllow ?? [],
      apiUrl: raw.apiUrl,
    };
  } catch {
    return null;
  }
}

function extractTargetPath(rawUrl: string, payload: ProcessGitViewerPayload): string | null {
  try {
    const url = new URL(rawUrl, window.location.origin);
    const prefix = `${payload.repoLink}/raw/`;
    if (!url.pathname.startsWith(prefix)) return null;
    const remainder = url.pathname.slice(prefix.length);
    const parts = remainder.split('/');
    if (parts.length < 3) return null;
    const pathParts = parts.slice(2).map((segment) => decodeURIComponent(segment));
    return pathParts.join('/');
  } catch {
    return null;
  }
}

function buildSaveUrl(payload: ProcessGitViewerPayload, treePath: string): string {
  return `${payload.repoLink}/_edit/${encodePath(payload.branch)}/${encodePath(treePath)}`;
}

async function fetchRawContent(rawUrl: string): Promise<string> {
  const resolvedUrl = new URL(rawUrl, window.location.origin);
  const response = await fetch(resolvedUrl.toString(), {credentials: 'same-origin'});
  if (!response.ok) {
    throw new Error(`Neizdevās ielādēt saturu (${response.status})`);
  }
  return response.text();
}

async function saveContent(payload: ProcessGitViewerPayload, treePath: string, content: string, summary: string) {
  if (!payload.branch || !payload.lastCommit) {
    throw new Error('Trūkst informācijas saglabāšanai (branch/commit).');
  }

  const form = new FormData();
  form.set('_csrf', window.config.csrfToken);
  form.set('last_commit', payload.lastCommit);
  form.set('commit_choice', 'direct');
  form.set('commit_summary', summary);
  form.set('commit_message', '');
  form.set('new_branch_name', '');
  form.set('tree_path', treePath);
  form.set('content', content);

  const response = await fetch(buildSaveUrl(payload, treePath), {
    method: 'POST',
    headers: {
      'X-Requested-With': 'XMLHttpRequest',
      Accept: 'application/json',
    },
    body: form,
  });

  if (!response.ok) {
    throw new Error(response.statusText || 'Failed to save ProcessGit file');
  }
  const json = await response.json().catch(() => ({}));
  if (json.error) throw new Error(json.error);
  if (json.redirect) {
    window.location.href = json.redirect;
    return;
  }
  showInfoToast('Izmaiņas saglabātas');
  window.location.reload();
}

let registered = false;

export function initRepoProcessGitViewer(): void {
  if (registered) return;
  registered = true;

  registerGlobalInitFunc('initRepoProcessGitViewer', async (container: HTMLElement) => {
    const mount = container.querySelector<HTMLElement>('#processgit-viewer-mount');
    const script = container.querySelector<HTMLScriptElement>('#processgit-viewer-payload');
    const payload = parsePayload(script);
    const rawPanelId = container.getAttribute('data-pgv-raw-panel') ?? 'diagram-raw-view';
    let rawPanel = document.getElementById(rawPanelId);
    if (!rawPanel) {
      rawPanel =
        document.getElementById('repo-file-content') ||
        document.querySelector<HTMLElement>('.file-view .markup') ||
        document.querySelector<HTMLElement>('.repository.file.list #repo-files-table') ||
        document.querySelector<HTMLElement>('.file-view') ||
        null;
    }
    console.log('[PGV] rawPanel resolved', {rawPanelId, found: Boolean(rawPanel)});
    const saveButton = container.querySelector<HTMLButtonElement>('[data-pgv-action="save"], [data-pgv-tab="save"]');
    const guiButton = container.querySelector<HTMLElement>('[data-pgv-action="gui"], [data-pgv-tab="gui"]');
    const rawButton = container.querySelector<HTMLElement>('[data-pgv-action="raw"], [data-pgv-tab="raw"]');

    if (!mount || !payload) return;

    const readAllow = new Set<string>([payload.path]);
    const rawByPath = new Map<string, string>();
    Object.values(payload.targets).forEach((rawUrl) => {
      const parsed = extractTargetPath(rawUrl, payload);
      if (parsed) {
        readAllow.add(parsed);
        rawByPath.set(parsed, rawUrl);
      }
    });

    mount.innerHTML = '';
    mount.style.height = 'calc(100vh - 260px)';
    const iframe = document.createElement('iframe');
    iframe.style.width = '100%';
    iframe.style.height = '100%';
    iframe.style.display = 'block';
    iframe.style.border = '0';
    iframe.classList.add('processgit-viewer-frame');
    iframe.setAttribute('sandbox', 'allow-scripts allow-forms');
    mount.append(iframe);

    const entryUrl = new URL(payload.entryRawUrl, window.location.origin);
    const baseHref = entryUrl.toString().replace(/[^/]*$/, '');

    iframe.addEventListener('load', () => {
      console.log('[PGV] iframe loaded', {entry: payload.entryRawUrl, baseHref});
    });

    window.addEventListener('message', async (ev) => {
      if (ev.source !== iframe.contentWindow) return;
      const msg = ev.data as any;
      if (!msg || typeof msg !== 'object') return;

      if (msg.type === 'PGV_FETCH' && typeof msg.url === 'string' && typeof msg.reqId === 'string') {
        try {
          const u = new URL(msg.url, window.location.origin);

          // Allow ONLY same-origin fetches
          if (u.origin !== window.location.origin) throw new Error('cross-origin blocked');

          const r = await fetch(u.toString(), {credentials: 'same-origin'});
          const text = await r.text();

          iframe.contentWindow?.postMessage({type: 'PGV_FETCH_RESULT', reqId: msg.reqId, url: msg.url, ok: true, text}, '*');
        } catch (e) {
          iframe.contentWindow?.postMessage({type: 'PGV_FETCH_RESULT', reqId: msg.reqId, url: msg.url, ok: false, error: String(e)}, '*');
        }
      }
    });

    try {
      const res = await fetch(entryUrl.toString(), {credentials: 'same-origin'});
      let htmlText = await res.text();
      htmlText = injectBaseTag(htmlText, baseHref);
      iframe.srcdoc = htmlText;
    } catch (error) {
      showErrorToast(toMessage(error));
    }

    const showGui = () => {
      if (rawPanel) rawPanel.style.display = 'none';
      mount.style.display = '';
    };

    const showRaw = () => {
      if (rawPanel) rawPanel.style.display = '';
      mount.style.display = 'none';
    };

    showGui();

    guiButton?.addEventListener('click', showGui);
    if (!rawPanel && rawButton) {
      rawButton.classList.add('disabled');
      rawButton.setAttribute('aria-disabled', 'true');
    }
    if (rawPanel) {
      rawButton?.addEventListener('click', () => {
        showRaw();
      });
    }

    const postToIframe = (message: Record<string, unknown> | string) => {
      iframe.contentWindow?.postMessage(message, '*');
    };

    saveButton?.addEventListener('click', () => {
      postToIframe({type: 'PGV_SAVE_CLICKED'});
    });

    const handleMessage = async (event: MessageEvent) => {
      if (event.source !== iframe.contentWindow) return;

      const data = event.data as {type?: string} | string;
      const type = typeof data === 'string' ? data : data?.type;

      switch (type) {
        case 'PGV_READY': {
          postToIframe({type: 'PGV_INIT', payload});
          break;
        }
        case 'PGV_DIRTY': {
          const dirty = typeof data === 'object' && data ? Boolean((data as {dirty?: boolean}).dirty) : false;
          if (saveButton) saveButton.disabled = !dirty;
          break;
        }
        case 'PGV_REQUEST_LOAD': {
          const requestedPath = typeof data === 'object' && data ? (data as {path?: string}).path ?? payload.path : payload.path;
          if (!readAllow.has(requestedPath)) {
            showErrorToast('Nav atļauts ielādēt pieprasīto failu.');
            postToIframe({type: 'PGV_LOAD_RESULT', path: requestedPath, content: ''});
            return;
          }
          try {
            const rawUrl = rawByPath.get(requestedPath);
            if (!rawUrl) {
              throw new Error('Nav pieejams neapstrādāts URL šim failam.');
            }
            const content = await fetchRawContent(rawUrl);
            postToIframe({type: 'PGV_LOAD_RESULT', path: requestedPath, content});
          } catch (error) {
            showErrorToast(toMessage(error));
            postToIframe({type: 'PGV_LOAD_RESULT', path: requestedPath, content: ''});
          }
          break;
        }
        case 'PGV_REQUEST_SAVE': {
          const request = typeof data === 'object' && data ? (data as {path?: string; content?: string; summary?: string}) : {};
          const requestedPath = request.path ?? payload.path;
          if (!payload.editAllow.includes(requestedPath)) {
            postToIframe({type: 'PGV_SAVE_RESULT', ok: false, error: 'Nav atļauts saglabāt šo failu.'});
            return;
          }
          if (typeof request.content !== 'string') {
            postToIframe({type: 'PGV_SAVE_RESULT', ok: false, error: 'Trūkst saglabājamā satura.'});
            return;
          }
          try {
            const summary = request.summary ?? `Update ${payload.path.split('/').pop()}`;
            await saveContent(payload, requestedPath, request.content, summary);
            postToIframe({type: 'PGV_SAVE_RESULT', ok: true});
          } catch (error) {
            postToIframe({type: 'PGV_SAVE_RESULT', ok: false, error: toMessage(error)});
            showErrorToast(toMessage(error));
          }
          break;
        }
        default:
          break;
      }
    };

    window.addEventListener('message', handleMessage);
  });
}
