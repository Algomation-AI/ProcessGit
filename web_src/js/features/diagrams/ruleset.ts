import type {DiagramAdapter} from './types.ts';

interface RulesetRule {
  id?: string;
  name?: string;
  when?: string;
  then?: string;
  priority?: number | string;
}

function normalizeRules(data: any): RulesetRule[] {
  if (!data) return [];
  if (Array.isArray(data)) return data;
  if (Array.isArray(data.rules)) return data.rules;
  if (Array.isArray(data.decisions)) return data.decisions;
  return [];
}

function createHeaderCell(text: string) {
  const th = document.createElement('th');
  th.textContent = text;
  return th;
}

export function createRulesetAdapter(canvas: HTMLElement): DiagramAdapter {
  return {
    async renderPreview(data: any) {
      const rules = normalizeRules(data);
      canvas.innerHTML = '';

      if (!rules.length) {
        const empty = document.createElement('div');
        empty.className = 'ui message';
        empty.textContent = 'No rules to display.';
        canvas.append(empty);
        return;
      }

      const filterWrapper = document.createElement('div');
      filterWrapper.className = 'ui input tw-w-full tw-mb-3';
      const filterInput = document.createElement('input');
      filterInput.type = 'search';
      filterInput.placeholder = 'Filter rules';
      filterWrapper.append(filterInput);

      const table = document.createElement('table');
      table.className = 'ui celled table compact';
      const thead = document.createElement('thead');
      const headerRow = document.createElement('tr');
      ['Name', 'When', 'Then', 'Priority'].forEach((title) => headerRow.append(createHeaderCell(title)));
      thead.append(headerRow);
      const tbody = document.createElement('tbody');

      const rows = rules.map((rule) => {
        const row = document.createElement('tr');
        const cells = [
          rule.name ?? rule.id ?? '',
          rule.when ?? '',
          rule.then ?? '',
          rule.priority?.toString() ?? '',
        ];
        cells.forEach((cellText) => {
          const td = document.createElement('td');
          td.textContent = cellText;
          row.append(td);
        });
        tbody.append(row);
        return row;
      });

      const applyFilter = () => {
        const term = filterInput.value.toLowerCase();
        rows.forEach((row) => {
          const text = row.textContent?.toLowerCase() ?? '';
          row.classList.toggle('tw-hidden', term !== '' && !text.includes(term));
        });
      };

      filterInput.addEventListener('input', applyFilter);
      applyFilter();

      table.append(thead, tbody);
      canvas.append(filterWrapper, table);
    },
  };
}
