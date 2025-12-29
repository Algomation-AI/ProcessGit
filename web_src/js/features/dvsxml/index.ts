import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {showErrorToast, showInfoToast} from '../../modules/toast.ts';
import {
  createClassificationViewer,
  parseClassificationScheme,
} from './classification.ts';
import {parseDocumentMetadata, renderDocumentMetadata} from './documents.ts';
import type {DVSXMLPayload} from './types.ts';

type DVSMode = 'preview' | 'edit' | 'raw';

function toMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function encodePath(path: string): string {
  return path
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

function buildSaveUrl(payload: DVSXMLPayload): string {
  return `${payload.repoLink}/_edit/${encodePath(payload.branch)}/${encodePath(payload.path)}`;
}

async function saveDVS(payload: DVSXMLPayload, content: string, summary: string) {
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
  form.set('tree_path', payload.path);
  form.set('content', content);

  const response = await fetch(buildSaveUrl(payload), {
    method: 'POST',
    headers: {
      'X-Requested-With': 'XMLHttpRequest',
      Accept: 'application/json',
    },
    body: form,
  });

  if (!response.ok) {
    throw new Error(response.statusText || 'Failed to save DVS XML');
  }
  const json = await response.json().catch(() => ({}));
  if (json.error) throw new Error(json.error);
  if (json.redirect) {
    window.location.href = json.redirect;
    return;
  }
  showInfoToast('DVS XML saglabāts');
  window.location.reload();
}

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
      branch: raw.branch ?? '',
      lastCommit: raw.lastCommit ?? '',
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
    const rawPanelId = container.getAttribute('data-dvs-raw-panel') ?? 'diagram-raw-view';
    const rawPanel = document.getElementById(rawPanelId);
    const saveButton = container.querySelector<HTMLButtonElement>('[data-dvs-action="save"]');
    const previewButton = container.querySelector<HTMLButtonElement>('[data-dvs-action="preview"]');
    const editButton = container.querySelector<HTMLButtonElement>('[data-dvs-action="edit"]');
    const rawButton = container.querySelector<HTMLButtonElement>('[data-dvs-action="raw"]');
    const modeButtons = [previewButton, editButton, rawButton].filter(Boolean) as HTMLButtonElement[];

    if (!mount || !payload || !rawPanel) return;

    const beforeUnloadHandler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };

    let mode: DVSMode = 'preview';
    let dirty = false;
    let classificationViewer: ReturnType<typeof createClassificationViewer> | null = null;
    let currentContentType = payload.type;

    const markDirty = () => {
      if (dirty) return;
      dirty = true;
      if (saveButton) saveButton.disabled = false;
      window.addEventListener('beforeunload', beforeUnloadHandler);
    };

    const clearDirty = () => {
      if (!dirty) return;
      dirty = false;
      if (saveButton) saveButton.disabled = true;
      window.removeEventListener('beforeunload', beforeUnloadHandler);
    };

    const toggleRawView = (showRaw: boolean) => {
      rawPanel.classList.toggle('tw-hidden', !showRaw);
      mount.classList.toggle('tw-hidden', showRaw);
    };

    const updateActiveButtons = (active?: HTMLButtonElement | null) => {
      modeButtons.forEach((btn) => btn?.classList.remove('active'));
      active?.classList.add('active');
    };

    const switchToPreview = () => {
      mode = 'preview';
      toggleRawView(false);
      updateActiveButtons(previewButton);
      if (saveButton) {
        saveButton.classList.add('tw-hidden');
        saveButton.disabled = true;
      }
      classificationViewer?.setMode('preview');
    };

    const switchToEdit = () => {
      mode = 'edit';
      toggleRawView(false);
      updateActiveButtons(editButton);
      if (saveButton) {
        saveButton.classList.remove('tw-hidden');
        saveButton.disabled = !dirty;
      }
      classificationViewer?.setMode('edit');
    };

    const switchToRaw = () => {
      mode = 'raw';
      toggleRawView(true);
      updateActiveButtons(rawButton);
      if (saveButton) {
        saveButton.classList.add('tw-hidden');
        saveButton.disabled = true;
      }
    };

    const handleSave = async () => {
      if (currentContentType !== 'dvs.classification-scheme' || !classificationViewer) {
        showErrorToast('Šo DVS tipu nevar saglabāt no skatītāja.');
        return;
      }
      try {
        const serialized = classificationViewer.serialize();
        await saveDVS(
          payload,
          serialized,
          `Update DVS classification scheme: ${payload.path.split('/').pop() ?? payload.path}`,
        );
        clearDirty();
      } catch (err) {
        showErrorToast(`Failed to save DVS XML: ${toMessage(err)}`);
      }
    };

    previewButton?.addEventListener('click', (e) => {
      e.preventDefault();
      switchToPreview();
    });
    editButton?.addEventListener('click', (e) => {
      e.preventDefault();
      switchToEdit();
    });
    rawButton?.addEventListener('click', (e) => {
      e.preventDefault();
      switchToRaw();
    });
    saveButton?.addEventListener('click', (e) => {
      e.preventDefault();
      handleSave();
    });

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
        case 'dvs.classification-scheme': {
          const model = parseClassificationScheme(xmlText, payload);
          classificationViewer = createClassificationViewer(mount, model, payload, markDirty);
          currentContentType = 'dvs.classification-scheme';
          break;
        }
        case 'dvs.document-metadata': {
          mount.replaceChildren();
          renderDocumentMetadata(mount, parseDocumentMetadata(xmlText, payload));
          currentContentType = 'dvs.document-metadata';
          break;
        }
        default:
          renderMessage(mount, `Nav skatītāja tipam: ${payload.type}`, true, payload.rawUrl);
          return;
      }
    } catch (err: unknown) {
      showErrorToast(err instanceof Error ? err.message : String(err));
      renderMessage(mount, 'Neizdevās parsēt DVS XML saturu.', true, payload.rawUrl);
      return;
    }

    switchToPreview();
  });
}
