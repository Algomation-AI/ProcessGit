import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {showErrorToast} from '../../modules/toast.ts';
import {parseClassificationScheme, renderClassificationScheme} from './classification.ts';
import {parseDocumentMetadata, renderDocumentMetadata} from './documents.ts';
import type {DVSXMLPayload} from './types.ts';

function parsePayload(script: HTMLScriptElement | null): DVSXMLPayload | null {
  if (!script?.textContent) return null;
  try {
    const raw = JSON.parse(script.textContent) as Partial<DVSXMLPayload>;
    if (!raw || typeof raw.type !== 'string' || typeof raw.path !== 'string' || typeof raw.apiUrl !== 'string') {
      return null;
    }
    return {
      type: raw.type,
      path: raw.path,
      ref: raw.ref ?? '',
      repoLink: raw.repoLink ?? '',
      rawUrl: raw.rawUrl,
      apiUrl: raw.apiUrl,
      namespace: raw.namespace,
      schemaLocation: raw.schemaLocation,
      meta: raw.meta ?? {},
    };
  } catch {
    return null;
  }
}

function renderMessage(target: HTMLElement, message: string, isError = false, rawUrl?: string) {
  target.replaceChildren();
  const box = document.createElement('div');
  box.className = isError ? 'ui negative message' : 'ui message';
  const content = document.createElement('div');
  content.textContent = message;
  box.append(content);
  if (rawUrl) {
    const link = document.createElement('a');
    link.href = rawUrl;
    link.rel = 'nofollow';
    link.textContent = 'Skatīt kā tekstu';
    box.append(link);
  }
  target.append(box);
}

async function fetchDVSContent(payload: DVSXMLPayload): Promise<string> {
  const apiUrl = new URL(payload.apiUrl, window.location.origin);
  apiUrl.searchParams.set('path', payload.path);
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
    throw new Error(`Neizdevās ielādēt DVS XML${detail ? detail : ''}`);
  }
  const data = (await response.json()) as {content?: string; error?: string};
  if (!data?.content) {
    throw new Error(data?.error || 'Trūkst XML satura');
  }
  return data.content;
}

let registered = false;

export function initRepoDVSXML(): void {
  if (registered) return;
  registered = true;

  registerGlobalInitFunc('initRepoDVSXML', async (container: HTMLElement) => {
    const mount = container.querySelector<HTMLElement>('#dvs-xml-viewer');
    const script = container.querySelector<HTMLScriptElement>('#dvsxml-payload');
    const payload = parsePayload(script);
    if (!mount || !payload) return;

    mount.replaceChildren();
    const loader = document.createElement('div');
    loader.className = 'ui active inline loader';
    mount.append(loader);

    let xmlText: string;
    try {
      xmlText = await fetchDVSContent(payload);
    } catch (err: unknown) {
      showErrorToast(err instanceof Error ? err.message : String(err));
      renderMessage(mount, 'Neizdevās ielādēt failu priekšskatījumam.', true, payload.rawUrl);
      return;
    }

    try {
      switch (payload.type) {
        case 'dvs.classification-scheme':
          renderClassificationScheme(mount, parseClassificationScheme(xmlText, payload));
          break;
        case 'dvs.document-metadata':
          renderDocumentMetadata(mount, parseDocumentMetadata(xmlText, payload));
          break;
        default:
          renderMessage(mount, `Nav skatītāja tipam: ${payload.type}`, true, payload.rawUrl);
          break;
      }
    } catch (err: unknown) {
      showErrorToast(err instanceof Error ? err.message : String(err));
      renderMessage(mount, 'Neizdevās parsēt DVS XML saturu.', true, payload.rawUrl);
    }
  });
}
