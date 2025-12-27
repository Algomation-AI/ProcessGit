import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {showErrorToast, showInfoToast} from '../../modules/toast.ts';
import type {DiagramAdapter, DiagramPayload, RawDiagramPayload} from './types.ts';

type DiagramMode = 'preview' | 'edit' | 'raw';

function toMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function b64ToUtf8(b64: string): string {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder('utf-8').decode(bytes);
}

function decodePayloadContent(payload: DiagramPayload): string {
  if (payload.encoding === 'base64') {
    const encoded = payload.content || payload.contentB64 || '';
    if (!encoded) throw new Error('Diagram content is empty.');
    return b64ToUtf8(encoded).trim();
  }
  return (payload.content ?? '').trim();
}

function encodePath(path: string): string {
  return path.split('/').map(encodeURIComponent).join('/');
}

function buildSaveUrl(payload: DiagramPayload): string {
  return `${payload.repoLink}/_edit/${encodePath(payload.branch)}/${encodePath(payload.path)}`;
}

function validateXml(type: string, xml: string): string | null {
  if (!xml.trim()) return 'Diagram output is empty.';
  const parser = new DOMParser();
  const doc = parser.parseFromString(xml, 'text/xml');
  if (doc.getElementsByTagName('parsererror').length) return 'Diagram XML is not valid.';

  const localName = doc.documentElement.localName.toLowerCase();
  switch (type) {
    case 'bpmn':
      return localName === 'definitions' ? null : 'BPMN root element must be <definitions>.';
    case 'cmmn':
      return localName === 'definitions' ? null : 'CMMN root element must be <definitions>.';
    case 'dmn':
      return localName === 'definitions' ? null : 'DMN root element must be <definitions>.';
    default:
      return null;
  }
}

async function createAdapter(type: string, canvas: HTMLElement, properties: HTMLElement | null): Promise<DiagramAdapter | null> {
  switch (type) {
    case 'bpmn': {
      const {createBpmnAdapter} = await import('./bpmn.ts');
      return createBpmnAdapter(canvas, properties);
    }
    case 'cmmn': {
      const {createCmmnAdapter} = await import('./cmmn.ts');
      return createCmmnAdapter(canvas, properties);
    }
    case 'dmn': {
      const {createDmnAdapter} = await import('./dmn.ts');
      return createDmnAdapter(canvas, properties);
    }
    case 'ngraph': {
      const {createNGraphAdapter} = await import('./ngraph.ts');
      return createNGraphAdapter(canvas);
    }
    case 'ruleset': {
      const {createRulesetAdapter} = await import('./ruleset.ts');
      return createRulesetAdapter(canvas);
    }
    default:
      return null;
  }
}

function normalizePayload(raw: RawDiagramPayload, container: HTMLElement): DiagramPayload | null {
  const type = raw.type ?? raw.Type ?? container.dataset.diagramType ?? '';
  const format = raw.format ?? raw.Format ?? container.dataset.diagramFormat ?? '';
  const content = raw.content ?? raw.contentB64 ?? raw.Content ?? '';
  const encoding = raw.encoding ?? raw.Encoding ?? (raw.contentB64 ? 'base64' : undefined);
  const contentB64 = raw.contentB64;

  if (!type) return null;

  return {
    ...raw,
    type,
    format,
    content,
    contentB64,
    encoding,
  } as DiagramPayload;
}

async function submitSave(payload: DiagramPayload, content: string) {
  const form = new FormData();
  form.set('_csrf', window.config.csrfToken);
  form.set('last_commit', payload.lastCommit);
  form.set('commit_choice', 'direct');
  form.set('commit_summary', `Update diagram ${payload.path}`);
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
    throw new Error(response.statusText || 'Failed to save diagram');
  }
  const json = await response.json().catch(() => ({}));
  if (json.redirect) {
    window.location.href = json.redirect;
    return;
  }
  if (json.error) {
    throw new Error(json.error);
  }
  showInfoToast('Diagram saved');
  window.location.reload();
}

function updateActiveButtons(buttons: HTMLButtonElement[], active?: HTMLButtonElement | null) {
  buttons.forEach((btn) => btn?.classList.remove('active'));
  active?.classList.add('active');
}

let registered = false;

export function initRepoDiagrams(): void {
  if (registered) return;
  registered = true;
  registerGlobalInitFunc('initRepoDiagrams', async (container: HTMLElement) => {
    const payloadElement = container.querySelector<HTMLScriptElement>('#diagram-payload');
    const canvas = container.querySelector<HTMLElement>('#diagram-canvas');
    const properties = container.querySelector<HTMLElement>('#diagram-properties');
    const stage = container.querySelector<HTMLElement>('.diagram-stage');
    const rawViewId = container.getAttribute('data-diagram-raw-panel') ?? 'diagram-raw-view';
    const rawView = document.getElementById(rawViewId);

    if (!payloadElement?.textContent || !canvas || !rawView) return;

    const fallbackToRaw = () => {
      rawView.classList.remove('tw-hidden');
      stage?.classList.add('tw-hidden');
    };

    let payload: DiagramPayload;
    try {
      const parsedPayload = JSON.parse(payloadElement.textContent) as RawDiagramPayload;
      const normalizedPayload = normalizePayload(parsedPayload, container);
      if (!normalizedPayload) {
        showErrorToast('Unable to read diagram type for preview.');
        fallbackToRaw();
        return;
      }
      payload = normalizedPayload;
    } catch {
      showErrorToast('Unable to read diagram data for preview.');
      fallbackToRaw();
      return;
    }

    let decodedContent: string;
    try {
      decodedContent = decodePayloadContent(payload);
    } catch (err) {
      showErrorToast(`Unable to read diagram content: ${toMessage(err)}`);
      fallbackToRaw();
      return;
    }
    if (payload.format === 'xml' && decodedContent && !decodedContent.startsWith('<')) {
      showErrorToast('Diagram content is not XML.');
      fallbackToRaw();
      return;
    }

    let workingContent: any = decodedContent;
    if (payload.format === 'json') {
      try {
        workingContent = JSON.parse(decodedContent);
      } catch {
        showErrorToast('Unable to parse diagram JSON content.');
        fallbackToRaw();
        return;
      }
    }

    const adapter = await createAdapter(payload.type, canvas, properties);
    if (!adapter) {
      showErrorToast(`No viewer available for diagram type "${payload.type}".`);
      fallbackToRaw();
      return;
    }

    let mode: DiagramMode = 'preview';
    let dirty = false;
    const saveButton = container.querySelector<HTMLButtonElement>('.diagram-save-button');
    const previewButton = container.querySelector<HTMLButtonElement>('[data-diagram-action="preview"]');
    const editButton = container.querySelector<HTMLButtonElement>('[data-diagram-action="edit"]');
    const rawButton = container.querySelector<HTMLButtonElement>('[data-diagram-action="raw"]');
    const modeButtons = [previewButton, editButton, rawButton].filter(Boolean) as HTMLButtonElement[];

    const beforeUnloadHandler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };

    const markDirty = () => {
      if (!payload.editable || dirty) return;
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
      rawView.classList.toggle('tw-hidden', !showRaw);
      stage?.classList.toggle('tw-hidden', showRaw);
      if (properties && showRaw) properties.classList.add('tw-hidden');
    };

    const switchToPreview = async () => {
      if (mode === 'edit' && adapter.save) {
        try {
          workingContent = await adapter.save();
        } catch (err) {
          showErrorToast(`Unable to render preview: ${toMessage(err)}`);
          fallbackToRaw();
          return;
        }
      }
      mode = 'preview';
      toggleRawView(false);
      updateActiveButtons(modeButtons, previewButton);
      if (properties) properties.classList.add('tw-hidden');
      if (saveButton) {
        saveButton.classList.add('tw-hidden');
        saveButton.disabled = true;
      }
      try {
        await adapter.renderPreview(workingContent);
      } catch (err) {
        showErrorToast(`Unable to render diagram: ${toMessage(err)}`);
        fallbackToRaw();
      }
    };

    const switchToEdit = async () => {
      if (!payload.editable || !adapter.enterEdit) return;
      mode = 'edit';
      toggleRawView(false);
      updateActiveButtons(modeButtons, editButton);
      if (properties) properties.classList.remove('tw-hidden');
      if (saveButton) {
        saveButton.classList.remove('tw-hidden');
        saveButton.disabled = true;
      }
      try {
        await adapter.enterEdit(workingContent);
        adapter.setChangeHandler?.(markDirty);
      } catch (err) {
        showErrorToast(`Unable to start edit mode: ${toMessage(err)}`);
        fallbackToRaw();
      }
    };

    const switchToRaw = () => {
      mode = 'raw';
      toggleRawView(true);
      updateActiveButtons(modeButtons, rawButton);
      if (saveButton) {
        saveButton.classList.add('tw-hidden');
        saveButton.disabled = true;
      }
    };

    const handleSave = async () => {
      if (!adapter.save) return;
      try {
        const serialized = await adapter.save();
        const validationError = payload.format === 'xml' ? validateXml(payload.type, serialized) : null;
        if (validationError) {
          showErrorToast(validationError);
          return;
        }
        await submitSave(payload, serialized);
        clearDirty();
      } catch (err) {
        showErrorToast(`Failed to save diagram: ${toMessage(err)}`);
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

    await switchToPreview();
  });
}
