import {fomanticQuery} from '../../modules/fomantic/base.ts';
import type {GraphNode} from './types.ts';

export interface UIHandlers {
  onSearch: (query: string) => void;
  onSelectNode: (nodeId: string | null) => void;
  onRename: () => void;
  onSetCardinality: () => void;
  onEditDocumentation: () => void;
  onAddChild: () => void;
  onExport: () => void;
  onToggleRaw: (showRaw: boolean) => void;
}

export interface XsdVisualUI {
  canvas: HTMLElement;
  updateProperties: (node: GraphNode | null) => void;
  updateNodeOptions: (nodes: GraphNode[]) => void;
  setSearchValue: (value: string) => void;
  setRawMode: (showRaw: boolean) => void;
}

function createButton(
  label: string,
  onClick: () => void,
  action?: string,
  className = 'ui button',
): HTMLButtonElement {
  const button = document.createElement('button');
  button.className = className;
  button.type = 'button';
  button.textContent = label;
  if (action) button.setAttribute('data-xsd-action', action);
  button.addEventListener('click', onClick);
  return button;
}

function createToolbar(handlers: UIHandlers) {
  const toolbar = document.createElement('div');
  toolbar.className = 'xsd-visual-toolbar tw-flex tw-flex-wrap tw-gap-2 tw-items-center tw-justify-between';

  const left = document.createElement('div');
  left.className = 'tw-flex tw-flex-wrap tw-gap-2 tw-items-center';

  const search = document.createElement('div');
  search.className = 'ui input';
  const searchInput = document.createElement('input');
  searchInput.type = 'search';
  searchInput.placeholder = 'Search nodes';
  searchInput.addEventListener('input', () => handlers.onSearch(searchInput.value));
  search.append(searchInput);

  const selectWrapper = document.createElement('div');
  selectWrapper.className = 'ui selection dropdown';
  const select = document.createElement('select');
  select.addEventListener('change', () => {
    const value = select.value;
    handlers.onSelectNode(value ? value : null);
  });
  selectWrapper.append(select);

  left.append(search, selectWrapper);

  const right = document.createElement('div');
  right.className = 'tw-flex tw-flex-wrap tw-gap-2 tw-items-center';
  right.append(
    createButton('Rename', handlers.onRename, 'rename'),
    createButton('Set cardinality', handlers.onSetCardinality, 'cardinality'),
    createButton('Edit documentation', handlers.onEditDocumentation, 'documentation'),
    createButton('Add child', handlers.onAddChild, 'add-child'),
    createButton('Export', handlers.onExport, 'export'),
    createButton('Save', () => {}, 'save', 'ui primary button'),
  );

  const rawToggle = document.createElement('div');
  rawToggle.className = 'ui buttons';
  const diagramButton = document.createElement('button');
  diagramButton.className = 'ui button active';
  diagramButton.type = 'button';
  diagramButton.textContent = 'Diagram';
  diagramButton.setAttribute('data-xsd-action', 'diagram');
  const rawButton = document.createElement('button');
  rawButton.className = 'ui button';
  rawButton.type = 'button';
  rawButton.textContent = 'Raw';
  rawButton.setAttribute('data-xsd-action', 'raw');
  rawToggle.append(diagramButton, rawButton);

  diagramButton.addEventListener('click', () => {
    diagramButton.classList.add('active');
    rawButton.classList.remove('active');
    handlers.onToggleRaw(false);
  });
  rawButton.addEventListener('click', () => {
    rawButton.classList.add('active');
    diagramButton.classList.remove('active');
    handlers.onToggleRaw(true);
  });

  toolbar.append(left, rawToggle, right);

  return {toolbar, searchInput, select, diagramButton, rawButton};
}

function createPropertiesPanel() {
  const panel = document.createElement('div');
  panel.className = 'xsd-visual-properties ui segment';
  const header = document.createElement('h4');
  header.textContent = 'Properties';
  const content = document.createElement('div');
  content.className = 'tw-flex tw-flex-col tw-gap-2';
  panel.append(header, content);
  return {panel, content};
}

export function buildXsdVisualUI(mount: HTMLElement, handlers: UIHandlers): XsdVisualUI {
  mount.replaceChildren();

  const shell = document.createElement('div');
  shell.className = 'xsd-visual-shell tw-flex tw-flex-col tw-gap-3';

  const {toolbar, searchInput, select, diagramButton, rawButton} = createToolbar(handlers);

  const layout = document.createElement('div');
  layout.className = 'xsd-visual-layout tw-flex tw-gap-3';

  const canvas = document.createElement('div');
  canvas.className = 'xsd-visual-canvas-panel ui segment tw-flex-1';
  canvas.style.minHeight = '70vh';
  canvas.style.height = '70vh';

  const {panel, content} = createPropertiesPanel();

  layout.append(canvas, panel);
  shell.append(toolbar, layout);
  mount.append(shell);

  const updateProperties = (node: GraphNode | null) => {
    content.replaceChildren();
    if (!node) {
      const empty = document.createElement('div');
      empty.textContent = 'Select a node to see its properties.';
      content.append(empty);
      return;
    }

    const addRow = (label: string, value?: string) => {
      const row = document.createElement('div');
      row.className = 'tw-flex tw-flex-col';
      const labelEl = document.createElement('strong');
      labelEl.textContent = label;
      const valueEl = document.createElement('span');
      valueEl.textContent = value ?? '—';
      row.append(labelEl, valueEl);
      content.append(row);
    };

    addRow('Name', node.label);
    if (node.meta.type) addRow('Type', node.meta.type);
    if (node.meta.base) addRow('Base', node.meta.base);
    if (node.meta.occurs) addRow('Occurs', node.meta.occurs);
    if (node.meta.documentation) addRow('Documentation', node.meta.documentation);
    if (node.meta.targetNamespace) addRow('Target namespace', node.meta.targetNamespace);
  };

  const updateNodeOptions = (nodes: GraphNode[]) => {
    select.replaceChildren();
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = 'Select element/type';
    select.append(placeholder);
    nodes
      .filter((node) => node.kind === 'element' || node.kind === 'type')
      .forEach((node) => {
        const option = document.createElement('option');
        option.value = node.id;
        option.textContent = `${node.label} (${node.kind})`;
        select.append(option);
      });
  };

  const setSearchValue = (value: string) => {
    searchInput.value = value;
  };

  const setRawMode = (showRaw: boolean) => {
    if (showRaw) {
      rawButton.classList.add('active');
      diagramButton.classList.remove('active');
    } else {
      diagramButton.classList.add('active');
      rawButton.classList.remove('active');
    }
  };

  return {
    canvas,
    updateProperties,
    updateNodeOptions,
    setSearchValue,
    setRawMode,
  };
}

export interface ModalField {
  name: string;
  label: string;
  type: 'text' | 'number' | 'textarea';
  value?: string;
  placeholder?: string;
}

export function openFormModal(title: string, fields: ModalField[], onSubmit: (values: Record<string, string>) => void) {
  const modal = document.createElement('div');
  modal.className = 'ui modal';

  const header = document.createElement('div');
  header.className = 'header';
  header.textContent = title;

  const content = document.createElement('div');
  content.className = 'content';

  const form = document.createElement('form');
  form.className = 'ui form';

  fields.forEach((field) => {
    const fieldWrapper = document.createElement('div');
    fieldWrapper.className = 'field';
    const label = document.createElement('label');
    label.textContent = field.label;

    let input: HTMLInputElement | HTMLTextAreaElement;
    if (field.type === 'textarea') {
      const textarea = document.createElement('textarea');
      textarea.value = field.value ?? '';
      textarea.placeholder = field.placeholder ?? '';
      input = textarea;
    } else {
      const inputEl = document.createElement('input');
      inputEl.type = field.type;
      inputEl.value = field.value ?? '';
      inputEl.placeholder = field.placeholder ?? '';
      input = inputEl;
    }
    input.name = field.name;
    fieldWrapper.append(label, input);
    form.append(fieldWrapper);
  });

  content.append(form);

  const actions = document.createElement('div');
  actions.className = 'actions';
  const cancel = document.createElement('div');
  cancel.className = 'ui cancel button';
  cancel.textContent = 'Cancel';
  const approve = document.createElement('div');
  approve.className = 'ui primary ok button';
  approve.textContent = 'Save';
  actions.append(cancel, approve);

  modal.append(header, content, actions);
  document.body.append(modal);

  const valuesFromForm = () => {
    const data = new FormData(form);
    return Object.fromEntries(Array.from(data.entries()).map(([k, v]) => [k, String(v)]));
  };

  fomanticQuery(modal).modal({
    onApprove: () => {
      onSubmit(valuesFromForm());
      return true;
    },
    onHidden: () => {
      modal.remove();
    },
  }).modal('show');
}

export function openExportModal(xsdText: string) {
  const modal = document.createElement('div');
  modal.className = 'ui modal';

  const header = document.createElement('div');
  header.className = 'header';
  header.textContent = 'Export XSD';

  const content = document.createElement('div');
  content.className = 'content';
  const pre = document.createElement('pre');
  pre.textContent = xsdText;
  pre.style.maxHeight = '60vh';
  pre.style.overflow = 'auto';
  pre.style.whiteSpace = 'pre-wrap';
  content.append(pre);

  const actions = document.createElement('div');
  actions.className = 'actions';
  const close = document.createElement('div');
  close.className = 'ui primary ok button';
  close.textContent = 'Close';
  actions.append(close);

  modal.append(header, content, actions);
  document.body.append(modal);

  fomanticQuery(modal).modal({
    onHidden: () => {
      modal.remove();
    },
  }).modal('show');
}
