import type {
  ClassificationCase,
  ClassificationGroup,
  ClassificationSchemeModel,
  DVSXMLPayload,
} from './types.ts';
import {
  childText,
  collectUnknownAttributes,
  collectUnknownChildren,
  findChild,
  findChildren,
  firstAttribute,
  trimToUndefined,
} from './utils.ts';

type ClassificationMode = 'preview' | 'edit';

interface ClassificationViewer {
  setMode(mode: ClassificationMode): void;
  serialize(): string;
  getModel(): ClassificationSchemeModel;
}

function parseCase(element: Element): ClassificationCase {
  const used = new Set<string>();
  const item: ClassificationCase = {
    unknownAttributes: {},
    unknownElements: [],
  };

  item.uid = firstAttribute(element, ['uid', 'uuid', 'id'], used);
  item.index = firstAttribute(element, ['index', 'indekss', 'kods', 'nr', 'numurs'], used);
  item.title = firstAttribute(element, ['nosaukums', 'title', 'name'], used);
  item.responsible = firstAttribute(element, ['atbildigais', 'atbildiba', 'iestade', 'responsible'], used);
  item.retention = firstAttribute(element, ['terminsglabat', 'glabatermins', 'retention'], used);
  item.retentionType = firstAttribute(element, ['terminsglabattips', 'glabatip', 'retentiontype'], used);
  item.environment = firstAttribute(element, ['vide', 'medium', 'environment'], used);
  item.system = firstAttribute(element, ['sistema', 'system', 'issaiste'], used);
  item.description = firstAttribute(element, ['apraksts', 'aprakstslieta', 'description'], used);

  item.index ??= childText(element, ['indekss', 'index', 'kods', 'nr', 'numurs']);
  item.title ??= childText(element, ['nosaukums', 'title', 'name']);
  item.responsible ??= childText(element, ['atbildigais', 'iestade', 'responsible']);
  item.retention ??= childText(element, ['terminsglabat', 'glabatermins', 'retention']);
  item.environment ??= childText(element, ['vide', 'medium', 'environment']);
  item.system ??= childText(element, ['sistema', 'system', 'issaiste']);
  item.description ??= childText(element, ['apraksts', 'description', 'komentars']);

  const allowed = new Set([
    'lieta',
    'nosaukums',
    'title',
    'name',
    'apraksts',
    'description',
    'komentars',
    'indekss',
    'index',
    'kods',
    'nr',
    'numurs',
    'terminsglabat',
    'glabatermins',
    'retention',
    'terminsglabattips',
    'glabatip',
    'retentiontype',
    'atbildigais',
    'responsible',
    'iestade',
    'vide',
    'medium',
    'environment',
    'sistema',
    'system',
    'issaiste',
  ]);
  item.unknownAttributes = collectUnknownAttributes(element, used);
  item.unknownElements = collectUnknownChildren(element, allowed);
  return item;
}

function parseGroup(element: Element): ClassificationGroup {
  const used = new Set<string>();
  const group: ClassificationGroup = {
    cases: [],
    unknownAttributes: {},
    unknownElements: [],
  };

  group.uid = firstAttribute(element, ['uid', 'uuid', 'id'], used);
  group.index = firstAttribute(element, ['index', 'indekss', 'kods', 'nr', 'numurs'], used);
  group.title = firstAttribute(element, ['nosaukums', 'title', 'name'], used);
  group.responsible = firstAttribute(element, ['atbildigais', 'atbildiba', 'iestade', 'responsible'], used);
  group.retention = firstAttribute(element, ['terminsglabat', 'glabatermins', 'retention'], used);
  group.description = firstAttribute(element, ['apraksts', 'description', 'komentars'], used);

  group.index ??= childText(element, ['indekss', 'index', 'kods', 'nr', 'numurs']);
  group.title ??= childText(element, ['nosaukums', 'title', 'name']);
  group.responsible ??= childText(element, ['atbildigais', 'responsible', 'iestade']);
  group.retention ??= childText(element, ['terminsglabat', 'glabatermins', 'retention']);
  group.description ??= childText(element, ['apraksts', 'description', 'komentars']);

  group.cases = findChildren(element, ['lieta']).map((c) => parseCase(c));

  const allowed = new Set([
    'lieta',
    'nosaukums',
    'title',
    'name',
    'apraksts',
    'description',
    'komentars',
    'indekss',
    'index',
    'kods',
    'nr',
    'numurs',
    'terminsglabat',
    'glabatermins',
    'retention',
    'atbildigais',
    'responsible',
    'iestade',
    'papildusinformacija',
  ]);
  group.unknownAttributes = collectUnknownAttributes(element, used);
  group.unknownElements = collectUnknownChildren(element, allowed);
  return group;
}

export function parseClassificationScheme(xmlText: string, payload: DVSXMLPayload): ClassificationSchemeModel {
  const parser = new DOMParser();
  const doc = parser.parseFromString(xmlText, 'application/xml');
  if (doc.getElementsByTagName('parsererror').length) {
    throw new Error('Classification scheme XML is not valid.');
  }

  const root = doc.documentElement;
  const used = new Set<string>();
  const namespace = payload.namespace ?? root.getAttribute('xmlns') ?? root.namespaceURI ?? undefined;
  const schemaLocation = payload.schemaLocation ?? firstAttribute(root, ['schemalocation'], used);
  const allowedRootChildren = new Set(['metadati', 'grupa', 'groups', 'metadata']);

  const metadati = findChild(root, ['metadati']);
  const periods = metadati ? findChild(metadati, ['periods', 'periodslist']) : null;

  const model: ClassificationSchemeModel = {
    type: 'dvs.classification-scheme',
    schemeId: firstAttribute(root, ['schemeid', 'shemaid', 'klasifikacijasid'], used),
    version: firstAttribute(root, ['version', 'versija'], used),
    generatedAt: firstAttribute(root, ['generatedat', 'izveidots', 'datums'], used),
    organization: {
      id: firstAttribute(root, ['iestadeorgid', 'iestadeid', 'orgid', 'iestadeskods'], used),
      name: firstAttribute(root, ['iestadenosaukums', 'iestade', 'organization', 'organizationname', 'nosaukums'], used),
    },
    period: {
      start: childText(periods, ['sakums', 'sakumsdatums', 'start', 'periodstart']),
      end: childText(periods, ['beigas', 'beigasdatums', 'end', 'periodend']),
    },
    namespace,
    schemaLocation,
    meta: payload.meta ?? {},
    groups: findChildren(root, ['grupa']).map((g) => parseGroup(g)),
    unknownAttributes: collectUnknownAttributes(root, used),
    unknownElements: collectUnknownChildren(root, allowedRootChildren),
    rootName: root.localName || undefined,
  };

  // fallback to payload metadata for IDs and names when XML is missing them
  model.schemeId = trimToUndefined(model.schemeId ?? payload.meta?.schemeId);
  model.version = trimToUndefined(model.version ?? payload.meta?.version);
  model.generatedAt = trimToUndefined(model.generatedAt ?? payload.meta?.generatedAt);
  if (model.organization) {
    model.organization.id = trimToUndefined(model.organization.id ?? payload.meta?.organizationId);
    model.organization.name = trimToUndefined(model.organization.name ?? payload.meta?.organizationName);
    if (!model.organization.id && !model.organization.name) {
      model.organization = undefined;
    }
  }

  return model;
}

function setAttributeIf(el: Element, name: string, value?: string) {
  if (!value) return;
  el.setAttribute(name, value);
}

function appendUnknownElements(doc: Document, parent: Element, unknownElements: string[]) {
  if (!unknownElements.length) return;
  const parser = new DOMParser();
  for (const raw of unknownElements) {
    const fragment = parser.parseFromString(`<wrapper>${raw}</wrapper>`, 'application/xml');
    if (fragment.getElementsByTagName('parsererror').length) {
      parent.append(doc.createTextNode(raw));
      continue;
    }
    const children = fragment.documentElement ? Array.from(fragment.documentElement.childNodes) : [];
    for (const child of children) {
      if (child.nodeType === Node.ELEMENT_NODE || child.nodeType === Node.TEXT_NODE || child.nodeType === Node.CDATA_SECTION_NODE) {
        parent.append(child.cloneNode(true));
      }
    }
  }
}

function addTextElement(doc: Document, parent: Element, name: string, value?: string) {
  if (!value) return;
  const el = doc.createElement(name);
  el.textContent = value;
  parent.append(el);
}

export function serializeClassificationScheme(model: ClassificationSchemeModel, payload: DVSXMLPayload): string {
  const doc = document.implementation.createDocument('', '', null);
  const rootName = model.rootName || 'KlasifikacijasShema';
  const root = doc.createElement(rootName);
  doc.append(root);

  const namespace = model.namespace ?? payload.namespace;
  if (namespace) {
    root.setAttribute('xmlns', namespace);
  }

  const schemaLocation = model.schemaLocation ?? payload.schemaLocation;
  if (schemaLocation) {
    root.setAttribute('xmlns:xsi', 'http://www.w3.org/2001/XMLSchema-instance');
    root.setAttribute('xsi:schemaLocation', schemaLocation);
  }

  setAttributeIf(root, 'schemeId', trimToUndefined(model.schemeId));
  setAttributeIf(root, 'version', trimToUndefined(model.version));
  setAttributeIf(root, 'generatedAt', trimToUndefined(model.generatedAt));
  setAttributeIf(root, 'iestadeOrgId', trimToUndefined(model.organization?.id));
  setAttributeIf(root, 'iestadeNosaukums', trimToUndefined(model.organization?.name));
  setAttributeIf(root, 'periodStart', trimToUndefined(model.period?.start));
  setAttributeIf(root, 'periodEnd', trimToUndefined(model.period?.end));

  for (const [attr, value] of Object.entries(model.unknownAttributes ?? {})) {
    setAttributeIf(root, attr, value);
  }

  for (const group of model.groups) {
    const groupEl = doc.createElement('Grupa');
    setAttributeIf(groupEl, 'uid', trimToUndefined(group.uid));
    setAttributeIf(groupEl, 'indekss', trimToUndefined(group.index));
    setAttributeIf(groupEl, 'nosaukums', trimToUndefined(group.title));
    setAttributeIf(groupEl, 'atbildigais', trimToUndefined(group.responsible));
    setAttributeIf(groupEl, 'terminsGlabat', trimToUndefined(group.retention));
    setAttributeIf(groupEl, 'apraksts', trimToUndefined(group.description));

    for (const [attr, value] of Object.entries(group.unknownAttributes ?? {})) {
      setAttributeIf(groupEl, attr, value);
    }

    for (const kase of group.cases) {
      const caseEl = doc.createElement('Lieta');
      setAttributeIf(caseEl, 'uid', trimToUndefined(kase.uid));
      setAttributeIf(caseEl, 'indekss', trimToUndefined(kase.index));
      setAttributeIf(caseEl, 'nosaukums', trimToUndefined(kase.title));
      setAttributeIf(caseEl, 'vide', trimToUndefined(kase.environment));
      setAttributeIf(caseEl, 'terminsGlabat', trimToUndefined(kase.retention));
      setAttributeIf(caseEl, 'terminsGlabatTips', trimToUndefined(kase.retentionType));
      setAttributeIf(caseEl, 'atbildigais', trimToUndefined(kase.responsible));
      setAttributeIf(caseEl, 'sistema', trimToUndefined(kase.system));

      for (const [attr, value] of Object.entries(kase.unknownAttributes ?? {})) {
        setAttributeIf(caseEl, attr, value);
      }

      addTextElement(doc, caseEl, 'Apraksts', trimToUndefined(kase.description));
      appendUnknownElements(doc, caseEl, kase.unknownElements ?? []);
      groupEl.append(caseEl);
    }

    appendUnknownElements(doc, groupEl, group.unknownElements ?? []);
    root.append(groupEl);
  }

  appendUnknownElements(doc, root, model.unknownElements ?? []);

  return new XMLSerializer().serializeToString(doc);
}

function createInputField(value: string | undefined, placeholder: string, onChange: (value: string) => void, disabled = false) {
  const wrapper = document.createElement('div');
  wrapper.className = 'ui input';
  const input = document.createElement('input');
  input.value = value ?? '';
  input.placeholder = placeholder;
  if (disabled) input.disabled = true;
  input.addEventListener('input', () => onChange(input.value));
  wrapper.append(input);
  return wrapper;
}

function createButton(label: string, className = 'ui button', handler?: () => void) {
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = className;
  btn.textContent = label;
  if (handler) btn.addEventListener('click', handler);
  return btn;
}

function matches(text: string | undefined, term: string) {
  if (!term) return true;
  return (text ?? '').toLowerCase().includes(term.toLowerCase());
}

function formatGroupTitle(group: ClassificationGroup) {
  const pieces = [trimToUndefined(group.index), trimToUndefined(group.title)].filter(Boolean);
  return pieces.join(' ') || 'Grupa';
}

function exportGroupAsCsv(group: ClassificationGroup, filter: string) {
  const rows = [['Indekss', 'Nosaukums', 'Termiņš', 'Vide', 'Atbildīgais', 'UID']];
  for (const kase of group.cases) {
    if (!matches(kase.index, filter) && !matches(kase.title, filter) && !matches(kase.description, filter)) continue;
    rows.push([
      kase.index ?? '',
      kase.title ?? '',
      kase.retention ?? '',
      kase.environment ?? '',
      kase.responsible ?? '',
      kase.uid ?? '',
    ]);
  }
  const csv = rows.map((cols) => cols.map((c) => `"${c.replace(/"/g, '""')}"`).join(',')).join('\n');
  const blob = new Blob([csv], {type: 'text/csv;charset=utf-8;'});
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = `${formatGroupTitle(group)}.csv`;
  link.click();
  URL.revokeObjectURL(link.href);
}

function openCaseDetailsModal(
  kase: ClassificationCase,
  mode: ClassificationMode,
  onSave: (updated: Partial<ClassificationCase>) => void,
) {
  const overlay = document.createElement('div');
  overlay.className =
    'tw-fixed tw-inset-0 tw-bg-black/50 tw-flex tw-items-center tw-justify-center tw-z-50';
  const modal = document.createElement('div');
  modal.className = 'ui segment tw-bg-white tw-rounded tw-shadow-lg tw-w-full sm:tw-w-[520px] tw-max-h-[80vh] tw-overflow-auto tw-space-y-4';

  const title = document.createElement('div');
  title.className = 'tw-text-lg tw-font-semibold';
  title.textContent = kase.title || kase.index || 'Lietas detaļas';
  modal.append(title);

  const grid = document.createElement('div');
  grid.className = 'tw-space-y-3';

  const addField = (
    label: string,
    value: string | undefined,
    placeholder: string,
    key: keyof ClassificationCase,
  ) => {
    const field = document.createElement('div');
    field.className = 'tw-flex tw-flex-col tw-gap-1';
    const lbl = document.createElement('div');
    lbl.className = 'tw-text-xs tw-uppercase tw-text-gray-600';
    lbl.textContent = label;
    field.append(lbl);
    const wrapper = document.createElement('div');
    wrapper.className = 'ui input';
    const input = document.createElement('input');
    input.value = value ?? '';
    input.placeholder = placeholder;
    input.disabled = mode === 'preview';
    input.addEventListener('input', () => onSave({[key]: input.value} as Partial<ClassificationCase>));
    wrapper.append(input);
    field.append(wrapper);
    grid.append(field);
  };

  addField('UID', kase.uid, 'UID', 'uid');
  addField('Atbildīgais', kase.responsible, 'Atbildīgā iestāde', 'responsible');
  addField('IS Saistība / Sistēma', kase.system, 'Sistēma', 'system');
  addField('Glabāšanas termiņa tips', kase.retentionType, 'Termiņa tips', 'retentionType');
  addField('Glabāšanas termiņš', kase.retention, 'Termiņš', 'retention');

  modal.append(grid);

  const footer = document.createElement('div');
  footer.className = 'tw-flex tw-justify-end tw-gap-2';
  const closeBtn = createButton('Aizvērt', 'ui button', () => {
    document.body.removeChild(overlay);
  });
  footer.append(closeBtn);

  if (mode === 'edit') {
    const saveBtn = createButton('Saglabāt', 'ui primary button', () => {
      document.body.removeChild(overlay);
    });
    footer.append(saveBtn);
  }

  modal.append(footer);
  overlay.append(modal);
  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) {
      document.body.removeChild(overlay);
    }
  });
  document.body.append(overlay);
}

export function createClassificationViewer(
  target: HTMLElement,
  model: ClassificationSchemeModel,
  payload: DVSXMLPayload,
  onDirty?: () => void,
): ClassificationViewer {
  const workingModel: ClassificationSchemeModel = JSON.parse(JSON.stringify(model));
  let mode: ClassificationMode = 'preview';
  let selectedGroup = 0;
  let groupFilter = '';
  let searchTerm = '';

  const markDirty = () => {
    onDirty?.();
  };

  const wrapper = document.createElement('div');
  wrapper.className = 'dvsxml-classification tw-flex tw-flex-col lg:tw-flex-row tw-gap-4';
  target.replaceChildren(wrapper);

  const sidebar = document.createElement('div');
  sidebar.className = 'dvsxml-sidebar ui segment tw-w-full lg:tw-w-72 tw-flex-shrink-0 tw-flex tw-flex-col tw-gap-3';
  const main = document.createElement('div');
  main.className = 'dvsxml-main tw-flex-1 tw-flex tw-flex-col tw-gap-4';

  wrapper.append(sidebar, main);

  const setMode = (newMode: ClassificationMode) => {
    mode = newMode;
    renderSidebar();
    renderMain();
  };

  const addGroup = () => {
    workingModel.groups.push({
      index: String(workingModel.groups.length + 1),
      title: 'Jauna grupa',
      cases: [],
      unknownAttributes: {},
      unknownElements: [],
    });
    selectedGroup = workingModel.groups.length - 1;
    markDirty();
    renderSidebar();
    renderMain();
  };

  const addCase = (group: ClassificationGroup) => {
    group.cases.push({
      index: String(group.cases.length + 1),
      title: 'Jauna lieta',
      unknownAttributes: {},
      unknownElements: [],
    });
    markDirty();
    renderMain();
  };

  const deleteCase = (group: ClassificationGroup, idx: number) => {
    group.cases.splice(idx, 1);
    markDirty();
    renderMain();
  };

  const renderGroupList = () => {
    const list = document.createElement('div');
    list.className = 'ui vertical menu dvsxml-group-list';
    const groups = workingModel.groups
      .map((group, idx) => ({group, idx}))
      .filter(({group}) => {
        if (!groupFilter) return true;
        const text = `${group.index ?? ''} ${group.title ?? ''}`.toLowerCase();
        return text.includes(groupFilter.toLowerCase());
      });

    if (!groups.length) {
      const empty = document.createElement('div');
      empty.className = 'item tw-text-sm tw-text-gray-600';
      empty.textContent = groupFilter ? 'Nav grupu' : 'Nav grupu šajā failā.';
      list.append(empty);
      return list;
    }

    for (const {group, idx} of groups) {
      const item = document.createElement('a');
      item.className = 'item';
      if (idx === selectedGroup) item.classList.add('active');
      item.textContent = formatGroupTitle(group);
      item.addEventListener('click', (e) => {
        e.preventDefault();
        selectedGroup = idx;
        renderSidebar();
        renderMain();
      });
      list.append(item);
    }
    return list;
  };

  const renderSidebar = () => {
    sidebar.replaceChildren();
    const title = document.createElement('div');
    title.className = 'tw-text-sm tw-font-semibold';
    title.textContent = 'Struktūra';
    sidebar.append(title);

    const filterWrapper = document.createElement('div');
    filterWrapper.className = 'ui icon input';
    const filterInput = document.createElement('input');
    filterInput.placeholder = 'Filtrēt grupas…';
    filterInput.value = groupFilter;
    filterInput.addEventListener('input', () => {
      groupFilter = filterInput.value;
      renderSidebar();
    });
    filterWrapper.append(filterInput);
    sidebar.append(filterWrapper);

    sidebar.append(renderGroupList());

    if (mode === 'edit') {
      const addBtn = createButton('+ Pievienot grupu', 'ui primary button', addGroup);
      sidebar.append(addBtn);
    }
  };

  const renderMeta = () => {
    const meta = document.createElement('div');
    meta.className = 'ui segment tw-space-y-2';
    const title = document.createElement('div');
    title.className = 'tw-text-lg tw-font-semibold';
    title.textContent = trimToUndefined(workingModel.organization?.name) ?? 'Klasifikācijas shēma';
    meta.append(title);

    const badges = document.createElement('div');
    badges.className = 'tw-flex tw-flex-wrap tw-gap-2 tw-text-sm';
    const addBadge = (label: string, value?: string) => {
      if (!value) return;
      const badge = document.createElement('span');
      badge.className = 'ui label';
      badge.textContent = `${label}: ${value}`;
      badges.append(badge);
    };
    addBadge('Shēmas ID', workingModel.schemeId);
    addBadge('Versija', workingModel.version);
    addBadge('Ģenerēts', workingModel.generatedAt);
    if (workingModel.organization?.id) addBadge('Iestādes ID', workingModel.organization.id);
    if (workingModel.period?.start || workingModel.period?.end) {
      addBadge('Periods', `${workingModel.period?.start ?? '—'} — ${workingModel.period?.end ?? '—'}`);
    }
    if (workingModel.namespace) {
      const ns = document.createElement('span');
      ns.className = 'tw-text-xs tw-text-gray-600';
      ns.textContent = workingModel.namespace;
      badges.append(ns);
    }
    if (workingModel.schemaLocation) {
      const schema = document.createElement('span');
      schema.className = 'tw-text-xs tw-text-gray-600';
      schema.textContent = workingModel.schemaLocation;
      badges.append(schema);
    }

    meta.append(badges);
    return meta;
  };

  const renderMain = () => {
    main.replaceChildren();
    main.append(renderMeta());

    if (!workingModel.groups.length) {
      const empty = document.createElement('div');
      empty.className = 'ui segment tw-text-sm tw-text-gray-600';
      empty.textContent = 'Nav grupu šajā failā.';
      main.append(empty);
      if (mode === 'edit') {
        const btn = createButton('+ Pievienot grupu', 'ui primary button', addGroup);
        main.append(btn);
      }
      return;
    }

    const group = workingModel.groups[selectedGroup] ?? workingModel.groups[0];
    if (!group) return;
    selectedGroup = workingModel.groups.indexOf(group);

    const groupHeader = document.createElement('div');
    groupHeader.className = 'ui segment tw-space-y-3';

    const titleRow = document.createElement('div');
    titleRow.className = 'tw-flex tw-flex-wrap tw-items-center tw-gap-3';

    if (mode === 'edit') {
      titleRow.append(
        createInputField(group.index, 'Indekss', (val) => {
          group.index = trimToUndefined(val);
          markDirty();
          renderSidebar();
        }),
      );
      titleRow.append(
        createInputField(group.title, 'Nosaukums', (val) => {
          group.title = trimToUndefined(val);
          markDirty();
          renderSidebar();
        }),
      );
    } else {
      const title = document.createElement('div');
      title.className = 'tw-text-xl tw-font-semibold';
      title.textContent = formatGroupTitle(group);
      titleRow.append(title);
    }

    if (group.responsible) {
      const badge = document.createElement('span');
      badge.className = 'ui label';
      badge.textContent = `Atbildīgais: ${group.responsible}`;
      titleRow.append(badge);
    }
    if (group.retention) {
      const badge = document.createElement('span');
      badge.className = 'ui label';
      badge.textContent = `Glab. termiņš: ${group.retention}`;
      titleRow.append(badge);
    }
    groupHeader.append(titleRow);

    if (mode === 'edit') {
      const extras = document.createElement('div');
      extras.className = 'tw-flex tw-flex-wrap tw-gap-3';
      extras.append(
        createInputField(group.responsible, 'Atbildīgais', (val) => {
          group.responsible = trimToUndefined(val);
          markDirty();
        }),
      );
      extras.append(
        createInputField(group.retention, 'Glabāšanas termiņš', (val) => {
          group.retention = trimToUndefined(val);
          markDirty();
        }),
      );
      groupHeader.append(extras);
    }

    if (group.description) {
      const desc = document.createElement('div');
      desc.className = 'tw-text-sm tw-text-gray-700';
      desc.textContent = group.description;
      groupHeader.append(desc);
    }

    main.append(groupHeader);

    const controls = document.createElement('div');
    controls.className = 'tw-flex tw-flex-wrap tw-gap-2 tw-items-center';
    const searchWrapper = document.createElement('div');
    searchWrapper.className = 'ui input';
    const searchInput = document.createElement('input');
    searchInput.placeholder = 'Meklēt lietās…';
    searchInput.value = searchTerm;
    searchInput.addEventListener('input', () => {
      searchTerm = searchInput.value;
      renderMain();
    });
    searchWrapper.append(searchInput);
    controls.append(searchWrapper);

    const clearBtn = createButton('Notīrīt', 'ui button', () => {
      searchTerm = '';
      renderMain();
    });
    controls.append(clearBtn);

    const exportBtn = createButton('Eksportēt CSV', 'ui button', () => exportGroupAsCsv(group, searchTerm));
    controls.append(exportBtn);

    main.append(controls);

    const table = document.createElement('table');
    table.className = 'ui celled table';
    const thead = document.createElement('thead');
    const headerRow = document.createElement('tr');
    ['Indekss', 'Nosaukums', 'Termiņš', 'Vide', ''].forEach((label) => {
      const th = document.createElement('th');
      th.textContent = label;
      headerRow.append(th);
    });
    thead.append(headerRow);
    table.append(thead);

    const tbody = document.createElement('tbody');
    const filteredCases = group.cases.filter(
      (kase) =>
        matches(kase.index, searchTerm) ||
        matches(kase.title, searchTerm) ||
        matches(kase.description, searchTerm) ||
        matches(kase.environment, searchTerm),
    );

    if (!filteredCases.length) {
      const row = document.createElement('tr');
      const cell = document.createElement('td');
      cell.colSpan = 5;
      cell.textContent = 'Nav lietu, kas atbilst meklēšanai.';
      row.append(cell);
      tbody.append(row);
    } else {
      for (const kase of filteredCases) {
        const row = document.createElement('tr');
        const caseIndex = group.cases.indexOf(kase);

        const addCell = (content: HTMLElement | string) => {
          const td = document.createElement('td');
          if (typeof content === 'string') {
            td.textContent = content;
          } else {
            td.append(content);
          }
          row.append(td);
        };

        if (mode === 'edit') {
          addCell(
            createInputField(kase.index, 'Indekss', (val) => {
              kase.index = trimToUndefined(val);
              markDirty();
            }),
          );
          addCell(
            createInputField(kase.title, 'Nosaukums', (val) => {
              kase.title = trimToUndefined(val);
              markDirty();
            }),
          );
          addCell(
            createInputField(kase.retention, 'Termiņš', (val) => {
              kase.retention = trimToUndefined(val);
              markDirty();
            }),
          );
          addCell(
            createInputField(kase.environment, 'Vide', (val) => {
              kase.environment = trimToUndefined(val);
              markDirty();
            }),
          );
        } else {
          addCell(trimToUndefined(kase.index) ?? '—');
          addCell(trimToUndefined(kase.title) ?? '—');
          addCell(trimToUndefined(kase.retention) ?? '—');
          addCell(trimToUndefined(kase.environment) ?? '—');
        }

        const actions = document.createElement('div');
        actions.className = 'tw-flex tw-gap-2';
        const detailsBtn = createButton('Detaļas', 'ui button', () =>
          openCaseDetailsModal(kase, mode, (updated) => {
            Object.assign(kase, updated);
            markDirty();
          }),
        );
        actions.append(detailsBtn);
        if (mode === 'edit') {
          const deleteBtn = createButton('', 'ui icon button', () => deleteCase(group, caseIndex));
          const icon = document.createElement('i');
          icon.className = 'trash icon';
          deleteBtn.append(icon);
          actions.append(deleteBtn);
        }
        addCell(actions);

        tbody.append(row);
      }
    }

    table.append(tbody);
    main.append(table);

    if (mode === 'edit') {
      const addCaseBtn = createButton('+ Pievienot lietu', 'ui primary button', () => addCase(group));
      main.append(addCaseBtn);
    }
  };

  renderSidebar();
  renderMain();

  return {
    setMode,
    serialize: () => serializeClassificationScheme(workingModel, payload),
    getModel: () => workingModel,
  };
}
