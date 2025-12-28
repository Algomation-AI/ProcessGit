import type {DocumentMetadataEntry, DocumentMetadataModel, DVSXMLPayload} from './types.ts';
import {
  childText,
  collectUnknownAttributes,
  collectUnknownChildren,
  findChild,
  findChildren,
  firstAttribute,
  trimToUndefined,
} from './utils.ts';

function parseDocumentEntry(element: Element): DocumentMetadataEntry {
  const used = new Set<string>();
  const entry: DocumentMetadataEntry = {
    fields: {},
    attributes: {},
    unknownElements: [],
  };

  entry.uid = firstAttribute(element, ['uid', 'uuid'], used);
  entry.id = firstAttribute(element, ['dokumentsid', 'dokumentsuid', 'docid', 'id'], used);
  entry.title = firstAttribute(element, ['nosaukums', 'title', 'name'], used) ?? childText(element, ['nosaukums', 'title', 'name']);
  entry.status = firstAttribute(element, ['statuss', 'status', 'stadija'], used) ?? childText(element, ['statuss', 'status', 'stadija']);
  entry.category = firstAttribute(element, ['kategorija', 'category'], used) ?? childText(element, ['kategorija', 'category']);
  entry.exportedAt = firstAttribute(element, ['eksportets', 'exportedat', 'eksportadatums'], used);
  entry.sourceSystem = firstAttribute(element, ['sistema', 'sistemanosaukums', 'source', 'system'], used);

  const allowedChildren = new Set([
    'nosaukums',
    'title',
    'name',
    'statuss',
    'status',
    'stadija',
    'kategorija',
    'category',
    'eksportets',
    'exportedat',
    'eksportadatums',
    'sistema',
    'sistemanosaukums',
    'source',
    'system',
  ]);

  for (const child of Array.from(element.children)) {
    const name = child.localName;
    const normalized = name.toLowerCase();
    if (allowedChildren.has(normalized)) continue;
    const value = trimToUndefined(child.textContent);
    if (value) {
      entry.fields[name] = value;
    }
  }

  entry.attributes = collectUnknownAttributes(element, used);
  entry.unknownElements = collectUnknownChildren(element, allowedChildren);
  return entry;
}

export function parseDocumentMetadata(xmlText: string, payload: DVSXMLPayload): DocumentMetadataModel {
  const parser = new DOMParser();
  const doc = parser.parseFromString(xmlText, 'application/xml');
  if (doc.getElementsByTagName('parsererror').length) {
    throw new Error('Document metadata XML is not valid.');
  }

  const root = doc.documentElement;
  const used = new Set<string>();
  const namespace = payload.namespace ?? root.getAttribute('xmlns') ?? root.namespaceURI ?? undefined;
  const schemaLocation = payload.schemaLocation ?? firstAttribute(root, ['schemalocation'], used);
  const exportedAt =
    firstAttribute(root, ['eksportets', 'exportedat', 'eksportadatums'], used) ??
    childText(root, ['eksportets', 'exportedat', 'eksportadatums']);
  const sourceSystem =
    firstAttribute(root, ['sistema', 'sistemanosaukums', 'source', 'system'], used) ??
    childText(root, ['sistema', 'sistemanosaukums', 'source', 'system']);

  const documentsRoot = findChild(root, ['dokumenti']) ?? root;
  const documents = findChildren(documentsRoot, ['dokuments', 'dokumentuieraksts', 'ieraksts']);
  const allowedRootChildren = new Set(['dokumenti', 'dokuments', 'dokumentuieraksts', 'ieraksts']);

  return {
    type: 'dvs.document-metadata',
    exportedAt,
    sourceSystem,
    namespace,
    schemaLocation,
    meta: payload.meta ?? {},
    documents: documents.map((d) => parseDocumentEntry(d)),
    unknownAttributes: collectUnknownAttributes(root, used),
    unknownElements: collectUnknownChildren(root, allowedRootChildren),
  };
}

export function renderDocumentMetadata(target: HTMLElement, model: DocumentMetadataModel): void {
  target.replaceChildren();

  const header = document.createElement('div');
  header.className = 'tw-flex tw-flex-col md:tw-flex-row tw-justify-between tw-gap-3 tw-mb-3';

  const title = document.createElement('div');
  title.className = 'tw-text-lg tw-font-semibold';
  title.textContent = 'DVS dokumentu metadati';
  header.append(title);

  const meta = document.createElement('div');
  meta.className = 'tw-flex tw-flex-wrap tw-gap-3 tw-text-sm';
  if (model.exportedAt) {
    const badge = document.createElement('span');
    badge.className = 'ui label';
    badge.textContent = `Eksportēts: ${model.exportedAt}`;
    meta.append(badge);
  }
  if (model.sourceSystem) {
    const badge = document.createElement('span');
    badge.className = 'ui label';
    badge.textContent = `Sistēma: ${model.sourceSystem}`;
    meta.append(badge);
  }
  if (model.namespace) {
    const ns = document.createElement('span');
    ns.className = 'tw-text-xs tw-text-gray-600';
    ns.textContent = model.namespace;
    meta.append(ns);
  }
  if (model.schemaLocation) {
    const schema = document.createElement('span');
    schema.className = 'tw-text-xs tw-text-gray-600';
    schema.textContent = model.schemaLocation;
    meta.append(schema);
  }
  header.append(meta);
  target.append(header);

  if (!model.documents.length) {
    const empty = document.createElement('div');
    empty.className = 'tw-text-sm tw-text-gray-600';
    empty.textContent = 'Nav dokumentu metadatu ierakstu.';
    target.append(empty);
    return;
  }

  const table = document.createElement('table');
  table.className = 'ui celled table';
  const thead = document.createElement('thead');
  const headerRow = document.createElement('tr');
  ['Nosaukums', 'ID', 'Statuss', 'Kategorija', 'Eksportēts', 'Papildu lauki'].forEach((titleText) => {
    const th = document.createElement('th');
    th.textContent = titleText;
    headerRow.append(th);
  });
  thead.append(headerRow);
  table.append(thead);

  const tbody = document.createElement('tbody');
  for (const entry of model.documents) {
    const row = document.createElement('tr');
    const fieldsSummary = Object.entries(entry.fields)
      .map(([key, value]) => `${key}: ${value}`)
      .join('; ');
    const cells = [
      entry.title ?? '—',
      entry.id ?? entry.uid ?? '—',
      entry.status ?? '—',
      entry.category ?? '—',
      entry.exportedAt ?? model.exportedAt ?? '—',
      fieldsSummary || '—',
    ];
    cells.forEach((val) => {
      const td = document.createElement('td');
      td.textContent = val;
      row.append(td);
    });
    tbody.append(row);
  }
  table.append(tbody);
  target.append(table);

  if (model.unknownElements.length || Object.keys(model.unknownAttributes).length) {
    const note = document.createElement('div');
    note.className = 'tw-text-xs tw-text-gray-600 tw-mt-2';
    note.textContent = 'Failā ir papildu lauki, kas tiek saglabāti bez attēlošanas.';
    target.append(note);
  }
}
