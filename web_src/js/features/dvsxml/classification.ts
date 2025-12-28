import type {ClassificationCase, ClassificationGroup, ClassificationSchemeModel, DVSXMLPayload} from './types.ts';
import {
	childText,
	collectUnknownAttributes,
	collectUnknownChildren,
	findChild,
	findChildren,
	firstAttribute,
	trimToUndefined,
} from './utils.ts';

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
  item.description = firstAttribute(element, ['apraksts', 'aprakstslieta', 'description'], used);

  item.index ??= childText(element, ['indekss', 'index', 'kods', 'nr', 'numurs']);
  item.title ??= childText(element, ['nosaukums', 'title', 'name']);
  item.responsible ??= childText(element, ['atbildigais', 'iestade', 'responsible']);
  item.retention ??= childText(element, ['terminsglabat', 'glabatermins', 'retention']);
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
    'atbildigais',
    'responsible',
    'iestade',
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

function addMetaItem(container: HTMLElement, label: string, value?: string, secondary?: string) {
  if (!value && !secondary) return;
  const wrapper = document.createElement('div');
  wrapper.className = 'tw-flex tw-flex-col tw-gap-1 tw-text-sm tw-border tw-rounded tw-p-3';

  const labelEl = document.createElement('div');
  labelEl.className = 'tw-text-xs tw-uppercase tw-text-gray-600';
  labelEl.textContent = label;
  wrapper.append(labelEl);

  if (value) {
    const valueEl = document.createElement('div');
    valueEl.className = 'tw-font-semibold';
    valueEl.textContent = value;
    wrapper.append(valueEl);
  }
  if (secondary) {
    const secondaryEl = document.createElement('div');
    secondaryEl.className = 'tw-text-xs tw-text-gray-600';
    secondaryEl.textContent = secondary;
    wrapper.append(secondaryEl);
  }

  container.append(wrapper);
}

function renderGroup(group: ClassificationGroup): HTMLElement {
  const card = document.createElement('div');
  card.className = 'tw-border tw-rounded tw-p-3 tw-flex tw-flex-col tw-gap-2';

  const header = document.createElement('div');
  header.className = 'tw-flex tw-justify-between tw-gap-2 tw-items-start';
  const title = document.createElement('div');
  title.className = 'tw-text-base tw-font-semibold';
  const pieces = [group.index, group.title].filter(Boolean);
  title.textContent = pieces.join(' — ') || 'Grupa';
  header.append(title);

  const badges = document.createElement('div');
  badges.className = 'tw-flex tw-gap-2 tw-items-center';
  if (group.retention) {
    const badge = document.createElement('span');
    badge.className = 'ui label';
    badge.textContent = `Glab. termiņš: ${group.retention}`;
    badges.append(badge);
  }
  if (group.responsible) {
    const badge = document.createElement('span');
    badge.className = 'ui label';
    badge.textContent = `Atbildīgais: ${group.responsible}`;
    badges.append(badge);
  }
  if (badges.childElementCount) header.append(badges);
  card.append(header);

  if (group.description) {
    const desc = document.createElement('div');
    desc.className = 'tw-text-sm tw-text-gray-700';
    desc.textContent = group.description;
    card.append(desc);
  }

  const caseArea = document.createElement('div');
  if (!group.cases.length) {
    const empty = document.createElement('div');
    empty.className = 'tw-text-sm tw-text-gray-600';
    empty.textContent = 'Nav lietu šajā grupā.';
    caseArea.append(empty);
  } else {
    const table = document.createElement('table');
    table.className = 'ui very basic compact table';
    const thead = document.createElement('thead');
    const headerRow = document.createElement('tr');
    ['Indekss', 'Nosaukums', 'Termiņš', 'Atbildīgais'].forEach((head) => {
      const th = document.createElement('th');
      th.textContent = head;
      headerRow.append(th);
    });
    thead.append(headerRow);
    table.append(thead);

    const tbody = document.createElement('tbody');
    for (const item of group.cases) {
      const row = document.createElement('tr');
      const cols = [
        item.index ?? '',
        item.title ?? '',
        item.retention ?? '',
        item.responsible ?? '',
      ];
      cols.forEach((val) => {
        const td = document.createElement('td');
        td.textContent = trimToUndefined(val) ?? '—';
        row.append(td);
      });
      tbody.append(row);
    }
    table.append(tbody);
    caseArea.append(table);
  }

  if (group.unknownElements.length || Object.keys(group.unknownAttributes).length) {
    const note = document.createElement('div');
    note.className = 'tw-text-xs tw-text-gray-600';
    note.textContent = 'Papildu elementi saglabāti (netiek rādīti).';
    card.append(note);
  }

  card.append(caseArea);
  return card;
}

export function renderClassificationScheme(target: HTMLElement, model: ClassificationSchemeModel): void {
  target.replaceChildren();

  const header = document.createElement('div');
  header.className = 'tw-flex tw-flex-col md:tw-flex-row tw-justify-between tw-gap-3 tw-mb-4';

  const titleArea = document.createElement('div');
  const title = document.createElement('div');
  title.className = 'tw-text-lg tw-font-semibold';
  title.textContent = trimToUndefined(model.organization?.name) ?? 'Klasifikācijas shēma';
  titleArea.append(title);

  if (model.organization?.id) {
    const orgId = document.createElement('div');
    orgId.className = 'tw-text-sm tw-text-gray-600';
    orgId.textContent = model.organization.id;
    titleArea.append(orgId);
  }

  const metaArea = document.createElement('div');
  metaArea.className = 'tw-grid sm:tw-grid-cols-2 lg:tw-grid-cols-3 tw-gap-3';
  addMetaItem(metaArea, 'Shēmas ID', model.schemeId);
  addMetaItem(metaArea, 'Versija', model.version);
  addMetaItem(metaArea, 'Ģenerēts', model.generatedAt);
  const periodValue =
    model.period?.start || model.period?.end
      ? `${model.period?.start ?? '—'} — ${model.period?.end ?? '—'}`
      : undefined;
  addMetaItem(metaArea, 'Periods', periodValue);
  addMetaItem(metaArea, 'Namespace', model.namespace, model.schemaLocation);

  header.append(titleArea, metaArea);
  target.append(header);

  const groupsArea = document.createElement('div');
  groupsArea.className = 'tw-flex tw-flex-col tw-gap-3';
  if (!model.groups.length) {
    const empty = document.createElement('div');
    empty.className = 'tw-text-sm tw-text-gray-600';
    empty.textContent = 'Nav atrastas grupas šajā failā.';
    groupsArea.append(empty);
  } else {
    model.groups.forEach((g) => groupsArea.append(renderGroup(g)));
  }
  target.append(groupsArea);

  if (model.unknownElements.length || Object.keys(model.unknownAttributes).length) {
    const note = document.createElement('div');
    note.className = 'tw-text-xs tw-text-gray-600 tw-mt-2';
    note.textContent = 'Failā ir elementi vai atribūti, kas netiek rādīti, bet tiks saglabāti.';
    target.append(note);
  }
}
