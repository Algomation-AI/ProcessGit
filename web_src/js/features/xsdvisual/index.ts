import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {showErrorToast, showInfoToast} from '../../modules/toast.ts';
import {addChildElement, renameElement, renameType, setDocumentation, setOccurs} from './editor.ts';
import {buildGraph, elementNodeId, typeNodeId} from './graph.ts';
import {parseXsd} from './parse.ts';
import {renderGraph} from './render.ts';
import {serializeXsd} from './serialize.ts';
import {buildXsdVisualUI, openExportModal, openFormModal} from './ui.ts';
import type {ComplexType, ElementDecl, ParsedXsd, SchemaDoc} from './types.ts';

interface XsdVisualPayload {
  id: string;
  path: string;
  branch: string;
  ref: string;
  lastCommit: string;
  repoLink: string;
  entryRawUrl: string;
  apiUrl: string;
  targets: Record<string, string>;
  editable: boolean;
}

function toMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function parsePayload(script: HTMLScriptElement | null): XsdVisualPayload | null {
  if (!script?.textContent) return null;
  try {
    const raw = JSON.parse(script.textContent) as Partial<XsdVisualPayload>;
    if (!raw || typeof raw.path !== 'string' || typeof raw.apiUrl !== 'string') return null;
    return {
      id: raw.id ?? 'xsd-visual',
      path: raw.path,
      branch: raw.branch ?? '',
      ref: raw.ref ?? '',
      lastCommit: raw.lastCommit ?? '',
      repoLink: raw.repoLink ?? '',
      entryRawUrl: raw.entryRawUrl ?? '',
      apiUrl: raw.apiUrl,
      targets: raw.targets ?? {},
      editable: raw.editable ?? false,
    };
  } catch {
    return null;
  }
}

async function fetchViaProxy(url: string): Promise<string> {
  if (window.parent && window.parent !== window) {
    const reqId = `xsd-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const response = new Promise<string>((resolve, reject) => {
      const handler = (event: MessageEvent) => {
        const data = event.data as {type?: string; reqId?: string; ok?: boolean; text?: string; error?: string};
        if (!data || data.type !== 'PGV_FETCH_RESULT' || data.reqId !== reqId) return;
        window.removeEventListener('message', handler);
        if (data.ok && typeof data.text === 'string') resolve(data.text);
        else reject(new Error(data.error ?? 'Failed to fetch content'));
      };
      window.addEventListener('message', handler);
    });

    window.parent.postMessage({type: 'PGV_FETCH', url, reqId}, '*');
    return response;
  }

  const response = await fetch(url, {credentials: 'same-origin'});
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.text();
}

async function loadXsdContent(payload: XsdVisualPayload): Promise<string> {
  const targetUrl = payload.targets.xsd ?? payload.targets.xml ?? payload.entryRawUrl;
  if (targetUrl) {
    return fetchViaProxy(new URL(targetUrl, window.location.origin).toString());
  }

  const apiUrl = new URL(payload.apiUrl, window.location.origin);
  apiUrl.searchParams.set('path', payload.path);
  if (payload.ref) apiUrl.searchParams.set('ref', payload.ref);

  const response = await fetch(apiUrl.toString(), {
    headers: {
      'X-Requested-With': 'XMLHttpRequest',
      Accept: 'application/json',
    },
  });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  const data = (await response.json()) as {content?: string; error?: string};
  if (!data.content) throw new Error(data.error ?? 'Missing XSD content');
  return data.content;
}

function renderError(mount: HTMLElement, message: string) {
  mount.replaceChildren();
  const box = document.createElement('div');
  box.className = 'ui negative message';
  box.textContent = message;
  mount.append(box);
}

function buildNodeLookup(doc: SchemaDoc) {
  const elementMap = new Map<string, ElementDecl>();
  const typeMap = new Map<string, ComplexType>();

  const recordElement = (element: ElementDecl, parent?: string) => {
    elementMap.set(elementNodeId(element.name, parent), element);
    element.children?.forEach((particle) => {
      if (particle.element) recordElement(particle.element, parent ?? element.name);
    });
  };

  doc.elements.forEach((element) => recordElement(element));
  doc.types.forEach((type) => {
    typeMap.set(typeNodeId(type.name), type);
    type.sequence?.forEach((particle) => {
      if (particle.element) recordElement(particle.element, type.name);
    });
    type.choice?.forEach((particle) => {
      if (particle.element) recordElement(particle.element, type.name);
    });
  });

  return {elementMap, typeMap};
}

async function saveXsd(payload: XsdVisualPayload, content: string) {
  if (!payload.branch || !payload.lastCommit) {
    throw new Error('Missing branch or commit info');
  }

  const encodePath = (path: string): string =>
    path
      .split('/')
      .map((segment) => encodeURIComponent(segment))
      .join('/');

  const form = new FormData();
  form.set('_csrf', window.config.csrfToken);
  form.set('last_commit', payload.lastCommit);
  form.set('commit_choice', 'direct');
  form.set('commit_summary', `Update ${payload.path.split('/').pop()}`);
  form.set('commit_message', '');
  form.set('new_branch_name', '');
  form.set('tree_path', payload.path);
  form.set('content', content);

  const response = await fetch(`${payload.repoLink}/_edit/${encodePath(payload.branch)}/${encodePath(payload.path)}`, {
    method: 'POST',
    headers: {
      'X-Requested-With': 'XMLHttpRequest',
      Accept: 'application/json',
    },
    body: form,
  });

  if (!response.ok) {
    throw new Error(response.statusText || 'Failed to save XSD');
  }
  const json = await response.json().catch(() => ({}));
  if (json.error) throw new Error(json.error);
  if (json.redirect) {
    window.location.href = json.redirect;
  }
}

let registered = false;

export function initRepoXsdVisual(): void {
  if (registered) return;
  registered = true;

  registerGlobalInitFunc('initRepoXsdVisual', async (container: HTMLElement) => {
    const mount = container.querySelector<HTMLElement>('#xsd-visual-mount');
    const script = container.querySelector<HTMLScriptElement>('#xsd-visual-payload');
    const payload = parsePayload(script);
    const rawPanelId = container.getAttribute('data-xsd-raw-panel') ?? 'diagram-raw-view';
    const rawPanel = document.getElementById(rawPanelId);

    if (!mount || !payload || !rawPanel) return;

    let parsed: ParsedXsd;
    try {
      const content = await loadXsdContent(payload);
      parsed = parseXsd(content);
    } catch (error) {
      renderError(mount, toMessage(error));
      return;
    }

    if (parsed.warnings.length) {
      showInfoToast(parsed.warnings.join('\n'));
    }

    let doc = parsed.doc;
    let model = buildGraph(doc);
    let nodeLookup = buildNodeLookup(doc);
    let selectedNodeId: string | null = null;
    let currentContent = serializeXsd(doc);
    let dirty = false;
    let renderer = renderGraph(document.createElement('div'), model, {onSelect: () => {}});

    const ui = buildXsdVisualUI(mount, {
      onSearch: (query) => renderer.setSearchQuery(query),
      onSelectNode: (nodeId) => selectNode(nodeId),
      onRename: () => handleRename(),
      onSetCardinality: () => handleCardinality(),
      onEditDocumentation: () => handleDocumentation(),
      onAddChild: () => handleAddChild(),
      onExport: () => openExportModal(currentContent),
      onToggleRaw: (showRaw) => toggleRaw(showRaw),
    });

    renderer.destroy();
    renderer = renderGraph(ui.canvas, model, {
      onSelect: (nodeId) => selectNode(nodeId),
    });

    const toggleRaw = (showRaw: boolean) => {
      rawPanel.classList.toggle('tw-hidden', !showRaw);
      mount.classList.toggle('tw-hidden', showRaw);
      ui.setRawMode(showRaw);
    };

    const selectNode = (nodeId: string | null) => {
      selectedNodeId = nodeId;
      renderer.setSelectedNode(nodeId);
      const node = nodeId ? model.nodeById.get(nodeId) ?? null : null;
      ui.updateProperties(node ?? null);
    };

    const refresh = () => {
      model = buildGraph(doc);
      nodeLookup = buildNodeLookup(doc);
      renderer.destroy();
      renderer = renderGraph(ui.canvas, model, {
        onSelect: (nodeId) => selectNode(nodeId),
      });
      ui.updateNodeOptions(model.nodes);
      if (selectedNodeId) {
        selectNode(selectedNodeId);
      }
    };

    const saveButton = container.querySelector<HTMLButtonElement>('[data-xsd-action="save"]');
    if (saveButton) saveButton.disabled = true;

    const markDirty = () => {
      if (dirty) return;
      dirty = true;
      if (saveButton) saveButton.disabled = false;
      window.postMessage({type: 'PGV_DIRTY', dirty: true}, '*');
    };

    const updateContent = () => {
      currentContent = serializeXsd(doc);
      window.postMessage({type: 'PGV_SET_CONTENT', path: payload.path, content: currentContent}, '*');
      markDirty();
    };

    const handleRename = () => {
      if (!selectedNodeId) return;
      const element = nodeLookup.elementMap.get(selectedNodeId);
      const type = nodeLookup.typeMap.get(selectedNodeId);
      if (!element && !type) return;

      openFormModal('Rename', [
        {
          name: 'name',
          label: 'New name',
          type: 'text',
          value: element?.name ?? type?.name ?? '',
        },
      ], (values) => {
        const newName = values.name?.trim();
        if (!newName) return;
        if (element) {
          renameElement(doc, element.name, newName);
          const parent = selectedNodeId?.startsWith('element:') && selectedNodeId.includes('/')
            ? selectedNodeId.slice('element:'.length).split('/')[0]
            : undefined;
          selectedNodeId = elementNodeId(newName, parent);
        }
        if (type) {
          renameType(doc, type.name, newName);
          selectedNodeId = typeNodeId(newName);
        }
        updateContent();
        refresh();
      });
    };

    const handleCardinality = () => {
      if (!selectedNodeId) return;
      const element = nodeLookup.elementMap.get(selectedNodeId);
      if (!element) return;

      openFormModal('Set cardinality', [
        {name: 'min', label: 'minOccurs', type: 'number', value: String(element.minOccurs ?? 1)},
        {name: 'max', label: 'maxOccurs', type: 'text', value: String(element.maxOccurs ?? 1)},
      ], (values) => {
        const min = values.min ? Number.parseInt(values.min, 10) : undefined;
        const max = values.max === 'unbounded' ? 'unbounded' : Number.parseInt(values.max, 10);
        setOccurs(
          element,
          Number.isFinite(min) ? min : undefined,
          Number.isFinite(max as number) ? max : values.max === 'unbounded' ? 'unbounded' : undefined,
        );
        updateContent();
        refresh();
      });
    };

    const handleDocumentation = () => {
      if (!selectedNodeId) return;
      const element = nodeLookup.elementMap.get(selectedNodeId);
      const type = nodeLookup.typeMap.get(selectedNodeId);
      if (!element && !type) return;

      openFormModal('Edit documentation', [
        {
          name: 'doc',
          label: 'Documentation',
          type: 'textarea',
          value: element?.annotation ?? type?.annotation ?? '',
        },
      ], (values) => {
        const docText = values.doc ?? '';
        if (element) setDocumentation(element, docText);
        if (type) setDocumentation(type, docText);
        updateContent();
        refresh();
      });
    };

    const handleAddChild = () => {
      if (!selectedNodeId) return;
      const type = nodeLookup.typeMap.get(selectedNodeId);
      if (!type) return;

      openFormModal('Add child element', [
        {name: 'name', label: 'Element name', type: 'text'},
        {name: 'type', label: 'Type (QName)', type: 'text'},
        {name: 'min', label: 'minOccurs', type: 'number', value: '1'},
        {name: 'max', label: 'maxOccurs', type: 'text', value: '1'},
      ], (values) => {
        const childName = values.name?.trim();
        const childType = values.type?.trim();
        if (!childName || !childType) return;
        const min = values.min ? Number.parseInt(values.min, 10) : undefined;
        const max = values.max === 'unbounded' ? 'unbounded' : Number.parseInt(values.max, 10);
        const ok = addChildElement(
          doc,
          type.name,
          childName,
          childType,
          Number.isFinite(min) ? min : undefined,
          Number.isFinite(max as number) ? max : values.max === 'unbounded' ? 'unbounded' : undefined,
        );
        if (!ok) {
          showErrorToast('Parent type does not have a sequence to add children.');
          return;
        }
        updateContent();
        refresh();
      });
    };

    ui.updateNodeOptions(model.nodes);
    ui.updateProperties(null);

    window.addEventListener('message', (event) => {
      const data = event.data as {type?: string; ok?: boolean};
      if (!data || data.type !== 'PGV_SAVE_RESULT') return;
      if (data.ok) {
        dirty = false;
        if (saveButton) saveButton.disabled = true;
        window.postMessage({type: 'PGV_DIRTY', dirty: false}, '*');
      }
    });
    if (saveButton && payload.editable) {
      saveButton.addEventListener('click', async () => {
        try {
          await saveXsd(payload, currentContent);
          window.postMessage({type: 'PGV_SAVE_RESULT', ok: true}, '*');
          showInfoToast('XSD saved');
        } catch (error) {
          window.postMessage({type: 'PGV_SAVE_RESULT', ok: false}, '*');
          showErrorToast(toMessage(error));
        }
      });
    }

    if (!payload.editable) {
      container.querySelectorAll<HTMLButtonElement>('[data-xsd-action]').forEach((button) => {
        const action = button.getAttribute('data-xsd-action');
        if (action && ['export', 'diagram', 'raw'].includes(action)) return;
        button.disabled = true;
      });
    }

    currentContent = serializeXsd(doc);
    window.postMessage({type: 'PGV_SET_CONTENT', path: payload.path, content: currentContent}, '*');
  });
}
