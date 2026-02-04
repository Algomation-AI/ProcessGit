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

function formatXml(xmlText: string): string {
  const parser = new DOMParser();
  const xmlDoc = parser.parseFromString(xmlText, 'application/xml');
  if (xmlDoc.getElementsByTagName('parsererror').length) {
    return xmlText;
  }

  const indentUnit = '  ';
  const output: string[] = [];

  const serializeNode = (node: Node, depth: number) => {
    const indent = indentUnit.repeat(depth);

    switch (node.nodeType) {
      case Node.DOCUMENT_NODE: {
        const doc = node as Document;
        if (doc.doctype) {
          output.push(`<!DOCTYPE ${doc.doctype.name}>`);
        }
        doc.childNodes.forEach((child) => serializeNode(child, depth));
        break;
      }
      case Node.ELEMENT_NODE: {
        const element = node as Element;
        const attrs = Array.from(element.attributes)
          .map((attr) => ` ${attr.name}="${attr.value}"`)
          .join('');
        const children = Array.from(element.childNodes);
        const textChildren = children.filter(
          (child) => child.nodeType === Node.TEXT_NODE && child.textContent?.trim(),
        );
        const hasElementChildren = children.some((child) => child.nodeType === Node.ELEMENT_NODE);

        if (!children.length) {
          output.push(`${indent}<${element.tagName}${attrs}/>`);
          break;
        }

        if (!hasElementChildren && textChildren.length === 1 && children.length === 1) {
          const text = textChildren[0].textContent?.trim() ?? '';
          output.push(`${indent}<${element.tagName}${attrs}>${text}</${element.tagName}>`);
          break;
        }

        output.push(`${indent}<${element.tagName}${attrs}>`);
        children.forEach((child) => {
          if (child.nodeType === Node.TEXT_NODE) {
            const text = child.textContent?.trim();
            if (text) {
              output.push(`${indent}${indentUnit}${text}`);
            }
            return;
          }
          if (child.nodeType === Node.CDATA_SECTION_NODE) {
            output.push(`${indent}${indentUnit}<![CDATA[${child.textContent ?? ''}]]>`);
            return;
          }
          if (child.nodeType === Node.COMMENT_NODE) {
            output.push(`${indent}${indentUnit}<!--${child.textContent ?? ''}-->`);
            return;
          }
          serializeNode(child, depth + 1);
        });
        output.push(`${indent}</${element.tagName}>`);
        break;
      }
      case Node.PROCESSING_INSTRUCTION_NODE: {
        output.push(`${indent}<?${node.nodeName} ${node.nodeValue ?? ''}?>`);
        break;
      }
      default:
        break;
    }
  };

  serializeNode(xmlDoc, 0);
  return `${output.join('\n')}\n`;
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
    const rawMount = container.querySelector<HTMLElement>('#processgit-raw-mount');
    const rawPre = container.querySelector<HTMLElement>('#processgit-raw-pre');
    const script = container.querySelector<HTMLScriptElement>('#processgit-viewer-payload');
    const payload = parsePayload(script);
    const saveButton = container.querySelector<HTMLButtonElement>('[data-pgv-action="save"], [data-pgv-tab="save"]');
    const guiButton = container.querySelector<HTMLElement>('[data-pgv-action="gui"], [data-pgv-tab="gui"]');
    const rawButton = container.querySelector<HTMLElement>('[data-pgv-action="raw"], [data-pgv-tab="raw"]');

    if (!mount || !rawMount || !rawPre || !payload) return;

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
    const iframe = document.createElement('iframe');
    iframe.style.width = '100%';
    iframe.style.height = 'calc(100vh - 240px)';
    iframe.style.display = 'block';
    iframe.style.border = '0';
    iframe.classList.add('processgit-viewer-frame');
    iframe.setAttribute('sandbox', 'allow-scripts allow-forms');
    mount.append(iframe);

    const entryUrl = new URL(payload.entryRawUrl, window.location.origin);
    const baseHref = entryUrl.toString().replace(/[^/]*$/, '');
    const primaryRawUrl = new URL(`${baseHref}${payload.path}`, window.location.origin).toString();

    iframe.addEventListener('load', () => {
      console.log('[PGV] iframe loaded', {entry: payload.entryRawUrl, baseHref});
    });

    window.addEventListener('message', async (ev) => {
      if (ev.source !== iframe.contentWindow) return;

      const msg = ev.data as {type?: string; url?: string; reqId?: string} | null;
      if (!msg || typeof msg !== 'object') return;

      if (msg.type === 'PGV_FETCH' && typeof msg.url === 'string') {
        try {
          const u = new URL(msg.url, window.location.origin);

          // Hard allow-list: only allow same-origin and only raw + src paths (tight security)
          if (u.origin !== window.location.origin) throw new Error('cross-origin blocked');
          if (!/\/(raw|src)\//.test(u.pathname)) throw new Error('path blocked');

          const r = await fetch(u.toString(), {credentials: 'same-origin'});
          if (!r.ok) throw new Error(`HTTP ${r.status}`);
          const text = await r.text();

          iframe.contentWindow?.postMessage(
            {type: 'PGV_FETCH_RESULT', reqId: msg.reqId, url: msg.url, ok: true, text},
            '*',
          );
        } catch (e) {
          iframe.contentWindow?.postMessage(
            {type: 'PGV_FETCH_RESULT', reqId: msg.reqId, url: msg.url, ok: false, error: String(e)},
            '*',
          );
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

    const setRawContent = (content: string) => {
      const maybeCodeMirror = (rawPre as unknown as {CodeMirror?: {setValue: (value: string) => void}}).CodeMirror;
      if (maybeCodeMirror?.setValue) {
        maybeCodeMirror.setValue(content);
        return;
      }
      rawPre.textContent = content;
    };

    const loadRawSource = async (): Promise<void> => {
      setRawContent('Loading...');
      try {
        const response = await fetch(primaryRawUrl, {credentials: 'same-origin'});
        let text = await response.text();
        if (payload.path.toLowerCase().endsWith('.xml')) {
          text = formatXml(text);
        }
        setRawContent(text);
      } catch (error) {
        setRawContent(`Failed to load raw source: ${toMessage(error)}`);
      }
    };

    const showGui = () => {
      rawMount.style.display = 'none';
      mount.style.display = '';
      guiButton?.classList.add('active');
      rawButton?.classList.remove('active');
    };

    const showRaw = async (): Promise<void> => {
      mount.style.display = 'none';
      rawMount.style.display = '';
      rawButton?.classList.add('active');
      guiButton?.classList.remove('active');
      await loadRawSource();
    };

    showGui();

    guiButton?.addEventListener('click', showGui);
    rawButton?.addEventListener('click', () => {
      void showRaw();
    });

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
