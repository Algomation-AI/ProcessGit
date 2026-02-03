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

async function fetchContent(payload: ProcessGitViewerPayload, treePath: string): Promise<string> {
  const apiUrl = new URL(payload.apiUrl, window.location.origin);
  apiUrl.searchParams.set('path', treePath);
  if (payload.ref) apiUrl.searchParams.set('ref', payload.ref);

  const response = await fetch(apiUrl.toString(), {
    headers: {
      'X-Requested-With': 'XMLHttpRequest',
      Accept: 'application/json',
    },
    method: 'GET',
  });

  if (!response.ok) {
    let detail = '';
    try {
      const errJson = await response.json();
      if (errJson?.error) detail = ` (${errJson.error})`;
    } catch {
      // ignore
    }
    throw new Error(`Neizdevās ielādēt saturu${detail}`);
  }

  const data = (await response.json()) as {content?: string; error?: string};
  if (!data?.content) {
    throw new Error(data?.error || 'Trūkst faila satura');
  }
  return data.content;
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
    const payloadEl = document.getElementById('processgit-viewer-payload');
    if (!payloadEl) return;

    const payload = JSON.parse(payloadEl.textContent || '{}') as ProcessGitViewerPayload;

    const mount = document.getElementById('processgit-viewer-mount');
    if (!mount) return;

    const rawPanelId = container.getAttribute('data-pgv-raw-panel') ?? 'diagram-raw-view';
    const rawPanel = document.getElementById(rawPanelId);
    const saveButton = container.querySelector<HTMLButtonElement>('[data-pgv-action="save"], [data-pgv-tab="save"]');
    const guiButton = container.querySelector<HTMLElement>('[data-pgv-action="gui"], [data-pgv-tab="gui"]');
    const rawButton = container.querySelector<HTMLElement>('[data-pgv-action="raw"], [data-pgv-tab="raw"]');

    if (!rawPanel) return;

    const readAllow = new Set<string>([payload.path]);
    Object.values(payload.targets).forEach((rawUrl) => {
      const parsed = extractTargetPath(rawUrl, payload);
      if (parsed) readAllow.add(parsed);
    });

    mount.innerHTML = '';
    const iframe = document.createElement('iframe');
    iframe.src = payload.entryRawUrl;
    iframe.style.width = '100%';
    iframe.style.minHeight = '900px';
    iframe.style.border = '0';
    iframe.classList.add('processgit-viewer-frame');
    mount.append(iframe);

    const toggleRawView = (showRaw: boolean) => {
      rawPanel.classList.toggle('tw-hidden', !showRaw);
      mount.classList.toggle('tw-hidden', showRaw);
      if (guiButton) guiButton.classList.toggle('active', !showRaw);
      if (rawButton) rawButton.classList.toggle('active', showRaw);
    };

    toggleRawView(false);

    guiButton?.addEventListener('click', () => toggleRawView(false));
    rawButton?.addEventListener('click', () => toggleRawView(true));

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
            const content = await fetchContent(payload, requestedPath);
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
