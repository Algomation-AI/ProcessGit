// UAPF v2.5.0+ Algorithm Card viewer.
//
// Renders the Preview tab for *.card.yaml / *.card.yml / *.card.json files.
// Polymorphic on the card's `implementation.type`:
//   - external → sample browser (string-equality match against embedded tests)
//   - inline   → per-language visualiser (regex highlight, FEEL evaluator,
//                dmn link-out to cornerstone, other languages fall back to
//                syntax-highlighted source + sample browser)
//   - composite → call-tree (v1.5 — currently falls back to summary)
//
// Common across all: metadata strip (kind / determinism / risk dot per
// UAPF chapter 13.10) + IO contract panel + sample browser footer.
//
// "Edit File" routes to Gitea's built-in editor (no in-page editor here).
// "View Source" uses the existing raw view.

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - js-yaml ships without types in some contexts
import yaml from 'js-yaml';
import type {DiagramAdapter} from './types.ts';

interface AlgorithmCard {
  kind?: string;
  id: string;
  name?: string;
  version?: string;
  intent?: string;
  description?: string;
  algorithm_kind?: string;
  determinism?: 'deterministic' | 'stochastic' | 'learned';
  io?: {
    inputs?: Array<{id: string; type: string; description?: string; unit?: string}>;
    outputs?: Array<{id: string; type: string; description?: string; unit?: string}>;
  };
  risk?: {
    aiActRiskClass?: string;
    humanOversight?: string;
    transparencyTier?: string;
  };
  implementation?: {
    type?: 'inline' | 'external' | 'composite';
    medium?: string;       // for external
    language?: string;     // for inline
    uri?: string;
    hash?: string;
    runtime?: Record<string, unknown>;
    inline?: {language?: string; source?: string};
    external?: {medium?: string; uri?: string};
    composite?: {composed_of?: Array<{id: string; description?: string}>};
  };
  tests?: Array<{
    name: string;
    description?: string;
    inputs: Record<string, unknown>;
    expected_outputs: Record<string, unknown>;
    tolerance?: Record<string, unknown>;
  }>;
  lifecycle?: {status?: string; since?: string};
  [k: string]: unknown;
}

// ── Risk-class derivation (UAPF chapter 13.10) ────────────────────
function riskClassFor(card: AlgorithmCard): 'green' | 'amber' | 'red' | 'unknown' {
  const ai = card.risk?.aiActRiskClass;
  const ov = card.risk?.humanOversight;
  const det = card.determinism;
  if (ai === 'high' || ai === 'unacceptable' || ov === 'mandatory') return 'red';
  if (det === 'stochastic' || det === 'learned') return 'amber';
  if (ai === 'limited' && ov && ov !== 'none') return 'amber';
  if (det === 'deterministic') return 'green';
  return 'unknown';
}

const RISK_DOT_COLOR: Record<string, string> = {
  green: '#1D9E75',
  amber: '#EF9F27',
  red: '#E24B4A',
  unknown: '#888780',
};

const RISK_LABEL: Record<string, string> = {
  green: 'low risk',
  amber: 'attention',
  red: 'high risk',
  unknown: 'unknown',
};

// ── Tiny DOM helpers (no framework dependency) ────────────────────
function el(tag: string, attrs: Record<string, string> = {}, ...children: (Node | string | null)[]): HTMLElement {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'class') e.className = v;
    else if (k === 'style') e.setAttribute('style', v);
    else if (k.startsWith('data-')) e.setAttribute(k, v);
    else e.setAttribute(k, v);
  }
  for (const c of children) {
    if (c == null) continue;
    e.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
  }
  return e;
}

function clear(container: HTMLElement): void {
  container.innerHTML = '';
}

function fmtValue(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (typeof v === 'object') return JSON.stringify(v, null, 2);
  return String(v);
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (typeof a !== typeof b) return false;
  if (a == null || b == null) return false;
  if (typeof a !== 'object') return false;
  const aa = a as Record<string, unknown>;
  const bb = b as Record<string, unknown>;
  const aKeys = Object.keys(aa);
  const bKeys = Object.keys(bb);
  if (aKeys.length !== bKeys.length) return false;
  for (const k of aKeys) if (!deepEqual(aa[k], bb[k])) return false;
  return true;
}

// ── Common header: metadata strip ─────────────────────────────────
function renderHeader(card: AlgorithmCard): HTMLElement {
  const risk = riskClassFor(card);
  const det = card.determinism || 'deterministic';
  const kind = card.algorithm_kind || 'algorithm';
  const impl = card.implementation || {};
  const implLabel = impl.type === 'external'
    ? `external · ${impl.medium || 'unknown'}`
    : impl.type === 'inline'
    ? `inline · ${(impl.inline && impl.inline.language) || impl.language || 'unknown'}`
    : impl.type === 'composite'
    ? `composite · ${(impl.composite?.composed_of || []).length} step(s)`
    : 'unknown';

  return el('div', {class: 'uapf-card-header', style: 'padding:16px 20px;background:#F8F7F4;border-bottom:1px solid #E5E2DC;'},
    // top row: id + risk dot
    el('div', {style: 'display:flex;align-items:center;justify-content:space-between;margin-bottom:8px;'},
      el('div', {style: 'font-family:\"Source Code Pro\",monospace;font-size:13px;color:#3D3929;font-weight:500;'}, card.id),
      el('div', {style: 'display:flex;align-items:center;gap:6px;font-size:11px;color:#7F7B6E;'},
        el('span', {style: `width:10px;height:10px;border-radius:50%;background:${RISK_DOT_COLOR[risk]};display:inline-block;`}),
        document.createTextNode(RISK_LABEL[risk]),
      ),
    ),
    // name
    card.name ? el('div', {style: 'font-size:18px;font-weight:600;color:#1F2328;margin-bottom:4px;'}, card.name) : null,
    // intent / description
    (card.intent || card.description)
      ? el('div', {style: 'font-size:13px;color:#5F5E5A;margin-bottom:10px;line-height:1.5;max-width:760px;'}, card.intent || card.description || '')
      : null,
    // metadata strip
    el('div', {style: 'display:flex;flex-wrap:wrap;gap:10px;font-size:12px;color:#5F5E5A;'},
      pill('v' + (card.version || '?')),
      pill(kind),
      pill(det),
      pill(implLabel),
      card.lifecycle?.status ? pill(card.lifecycle.status) : null,
    ),
  );
}

function pill(text: string | null): HTMLElement | null {
  if (!text) return null;
  return el('span', {style: 'background:#FFFFFF;border:1px solid #E5E2DC;border-radius:3px;padding:2px 8px;font-family:\"Source Code Pro\",monospace;font-size:11px;'}, text);
}

// ── Common header: IO contract panel ──────────────────────────────
function renderIOContract(card: AlgorithmCard): HTMLElement {
  const inputs = card.io?.inputs || [];
  const outputs = card.io?.outputs || [];
  return el('div', {style: 'padding:16px 20px;border-bottom:1px solid #E5E2DC;'},
    el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, 'IO contract'),
    el('div', {style: 'display:grid;grid-template-columns:1fr 1fr;gap:24px;'},
      // inputs
      el('div', {},
        el('div', {style: 'font-size:11px;color:#5F5E5A;margin-bottom:6px;'}, `inputs (${inputs.length})`),
        ...inputs.map(f =>
          el('div', {style: 'display:flex;justify-content:space-between;font-size:12px;padding:4px 8px;background:#F8F7F4;border-radius:3px;margin-bottom:2px;font-family:\"Source Code Pro\",monospace;'},
            el('span', {style: 'color:#3D3929;'}, f.id),
            el('span', {style: 'color:#888780;'}, `${f.type}${f.unit ? ' (' + f.unit + ')' : ''}`),
          ),
        ),
      ),
      // outputs
      el('div', {},
        el('div', {style: 'font-size:11px;color:#5F5E5A;margin-bottom:6px;'}, `outputs (${outputs.length})`),
        ...outputs.map(f =>
          el('div', {style: 'display:flex;justify-content:space-between;font-size:12px;padding:4px 8px;background:#F8F7F4;border-radius:3px;margin-bottom:2px;font-family:\"Source Code Pro\",monospace;'},
            el('span', {style: 'color:#3D3929;'}, f.id),
            el('span', {style: 'color:#888780;'}, `${f.type}${f.unit ? ' (' + f.unit + ')' : ''}`),
          ),
        ),
      ),
    ),
  );
}

// ── External sample browser (chapter 13.16, option a) ─────────────
//
// User edits the input fields. On each change we look for a test case
// whose `inputs` match exactly. If found, show the expected_outputs.
// Otherwise show "no recorded sample" + dropdown of all test cases.
function renderExternalSampleBrowser(card: AlgorithmCard): HTMLElement {
  const tests = card.tests || [];
  const inputs = card.io?.inputs || [];
  const outputs = card.io?.outputs || [];

  // State: currently selected test index, current input values
  let selectedIdx = 0;
  let currentInputs: Record<string, unknown> = {...(tests[0]?.inputs || {})};

  const root = el('div', {style: 'padding:16px 20px;'});

  // Section header + test selector
  const header = el('div', {style: 'display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;'},
    el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;'}, `sample browser (${tests.length} recorded)`),
    el('div', {style: 'font-size:11px;color:#888780;'}, 'external · black box · input → recorded output'),
  );
  root.appendChild(header);

  if (tests.length === 0) {
    root.appendChild(el('div', {style: 'padding:16px;background:#FEF8E7;border:1px solid #EBD995;border-radius:3px;color:#7A6112;font-size:13px;'},
      'This card has no embedded tests. SEM-014 violation (UAPF v2.5.0 chapter 11.3.6 requires ≥2 tests).'));
    return root;
  }

  // Test-case selector tabs
  const tabsRow = el('div', {style: 'display:flex;gap:4px;margin-bottom:12px;flex-wrap:wrap;'});
  const tabButtons: HTMLElement[] = [];
  tests.forEach((t, i) => {
    const btn = el('button', {
      type: 'button',
      style: `padding:6px 12px;border:1px solid #E5E2DC;background:${i === 0 ? '#3D3929' : '#FFFFFF'};color:${i === 0 ? '#FFFFFF' : '#3D3929'};border-radius:3px;font-size:12px;cursor:pointer;`,
    }, t.name);
    btn.addEventListener('click', () => selectTest(i));
    tabButtons.push(btn);
    tabsRow.appendChild(btn);
  });
  root.appendChild(tabsRow);

  // Two-column layout: inputs editor on left, outputs panel on right
  const grid = el('div', {style: 'display:grid;grid-template-columns:1fr 1fr;gap:16px;'});
  const inputsCol = el('div', {},
    el('div', {style: 'font-size:11px;color:#5F5E5A;margin-bottom:6px;'}, 'inputs (editable)'),
  );
  const outputsCol = el('div', {},
    el('div', {style: 'font-size:11px;color:#5F5E5A;margin-bottom:6px;'}, 'expected outputs'),
  );
  grid.appendChild(inputsCol);
  grid.appendChild(outputsCol);
  root.appendChild(grid);

  // Test description display
  const descBox = el('div', {style: 'margin-top:12px;padding:10px;background:#F8F7F4;border-left:3px solid #C5C3BB;font-size:12px;color:#5F5E5A;line-height:1.5;'});
  root.appendChild(descBox);

  // Input editor (one row per io.input)
  const inputElems = new Map<string, HTMLTextAreaElement>();
  inputs.forEach(f => {
    const inputId = `uapf-input-${f.id}`;
    const wrap = el('div', {style: 'margin-bottom:8px;'},
      el('label', {for: inputId, style: 'display:block;font-family:\"Source Code Pro\",monospace;font-size:11px;color:#3D3929;margin-bottom:2px;'},
        f.id, el('span', {style: 'color:#888780;font-weight:normal;margin-left:6px;'}, f.type)),
    );
    const isLong = f.type === 'string' && (typeof currentInputs[f.id] === 'string' && (currentInputs[f.id] as string).length > 60);
    const textarea = el('textarea', {
      id: inputId,
      style: `width:100%;min-height:${isLong ? '80px' : '38px'};padding:6px 8px;border:1px solid #C5C3BB;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:12px;color:#1F2328;resize:vertical;background:#FFFFFF;`,
    }) as HTMLTextAreaElement;
    inputElems.set(f.id, textarea);
    textarea.addEventListener('input', onInputChange);
    wrap.appendChild(textarea);
    inputsCol.appendChild(wrap);
  });

  // Outputs panel
  const outputBox = el('div', {style: 'background:#F8F7F4;padding:12px;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:12px;color:#3D3929;line-height:1.6;min-height:120px;white-space:pre-wrap;word-break:break-word;'});
  outputsCol.appendChild(outputBox);

  // Match-status badge
  const matchBadge = el('div', {style: 'margin-top:10px;font-size:11px;'});
  outputsCol.appendChild(matchBadge);

  function selectTest(i: number) {
    selectedIdx = i;
    tabButtons.forEach((b, j) => {
      b.style.background = j === i ? '#3D3929' : '#FFFFFF';
      b.style.color = j === i ? '#FFFFFF' : '#3D3929';
    });
    const t = tests[i];
    currentInputs = {...t.inputs};
    inputs.forEach(f => {
      const ta = inputElems.get(f.id);
      if (ta) {
        const v = currentInputs[f.id];
        ta.value = (typeof v === 'object' && v !== null) ? JSON.stringify(v, null, 2) : fmtValue(v);
      }
    });
    descBox.textContent = t.description || '(no description)';
    renderOutputs();
  }

  function onInputChange() {
    // Read all current values into currentInputs (parse JSON for object-typed inputs)
    inputs.forEach(f => {
      const ta = inputElems.get(f.id);
      if (!ta) return;
      const raw = ta.value;
      if (f.type === 'integer') currentInputs[f.id] = Number(raw);
      else if (f.type === 'boolean') currentInputs[f.id] = raw === 'true';
      else if (f.type === 'object' || f.type === 'array') {
        try { currentInputs[f.id] = JSON.parse(raw); }
        catch { currentInputs[f.id] = raw; }
      } else {
        currentInputs[f.id] = raw;
      }
    });
    renderOutputs();
  }

  function renderOutputs() {
    // Find a test whose inputs deep-equal currentInputs
    const matchIdx = tests.findIndex(t => deepEqual(t.inputs, currentInputs));
    if (matchIdx >= 0) {
      const t = tests[matchIdx];
      const lines: string[] = [];
      outputs.forEach(f => {
        const v = t.expected_outputs[f.id];
        lines.push(`${f.id}: ${fmtValue(v)}`);
      });
      outputBox.textContent = lines.join('\n');
      matchBadge.innerHTML = '';
      matchBadge.appendChild(
        el('span', {style: 'color:#1D9E75;'}, `✓ exact match: case "${t.name}"`));
    } else {
      outputBox.textContent = '(no recorded sample for these inputs — pick a recorded case above)';
      matchBadge.innerHTML = '';
      matchBadge.appendChild(
        el('span', {style: 'color:#A48F2A;'}, '◯ external card — outputs are recorded samples only, not predicted'));
    }
  }

  // Initial render
  selectTest(0);
  return root;
}

// ── Inline regex visualiser (chapter 13.16) ──────────────────────
function renderInlineRegex(card: AlgorithmCard): HTMLElement {
  const inline = card.implementation?.inline || {};
  const pattern = inline.source || '';
  const tests = card.tests || [];

  const root = el('div', {style: 'padding:16px 20px;'});
  root.appendChild(el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, 'inline · regex'));
  root.appendChild(el('pre', {style: 'background:#1F2328;color:#E8E5DA;padding:12px;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:13px;overflow-x:auto;'}, pattern));

  // For each test, show the input text with matches highlighted
  if (tests.length > 0) {
    let re: RegExp | null = null;
    try { re = new RegExp(pattern, 'g'); } catch (err) {
      root.appendChild(el('div', {style: 'color:#E24B4A;font-size:12px;margin-top:8px;'}, `Invalid regex: ${(err as Error).message}`));
      return root;
    }
    const firstInputKey = Object.keys(tests[0].inputs)[0];
    tests.forEach(t => {
      const text = String(t.inputs[firstInputKey] || '');
      root.appendChild(el('div', {style: 'margin-top:14px;'},
        el('div', {style: 'font-size:12px;color:#5F5E5A;margin-bottom:4px;'}, t.name),
        renderHighlighted(text, re!),
      ));
    });
  }
  return root;
}

function renderHighlighted(text: string, re: RegExp): HTMLElement {
  const out = el('div', {style: 'background:#F8F7F4;padding:10px;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:12px;line-height:1.6;white-space:pre-wrap;word-break:break-word;'});
  let lastIdx = 0;
  re.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index > lastIdx) out.appendChild(document.createTextNode(text.slice(lastIdx, m.index)));
    out.appendChild(el('span', {style: 'background:#FFE8B3;color:#7A6112;padding:0 2px;border-radius:2px;'}, m[0]));
    lastIdx = m.index + m[0].length;
    if (m[0].length === 0) re.lastIndex++;
  }
  if (lastIdx < text.length) out.appendChild(document.createTextNode(text.slice(lastIdx)));
  return out;
}

// ── Inline FEEL visualiser (placeholder — full evaluator is v1.5) ─
function renderInlineFeel(card: AlgorithmCard): HTMLElement {
  const inline = card.implementation?.inline || {};
  const expr = inline.source || '';
  const root = el('div', {style: 'padding:16px 20px;'});
  root.appendChild(el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, 'inline · FEEL'));
  root.appendChild(el('pre', {style: 'background:#1F2328;color:#E8E5DA;padding:12px;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:13px;overflow-x:auto;'}, expr));
  root.appendChild(el('div', {style: 'margin-top:12px;padding:10px;background:#FEF8E7;border:1px solid #EBD995;border-radius:3px;color:#7A6112;font-size:12px;'},
    'Live FEEL evaluation is v1.5 (planned). Use the sample browser below to see recorded input/output pairs.'));
  return root;
}

// ── Inline DMN link-out ──────────────────────────────────────────
function renderInlineDmnLinkOut(card: AlgorithmCard): HTMLElement {
  const root = el('div', {style: 'padding:16px 20px;'});
  root.appendChild(el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, 'inline · DMN'));
  root.appendChild(el('div', {style: 'padding:12px;background:#F8F7F4;border-left:3px solid #C5C3BB;font-size:13px;color:#3D3929;line-height:1.5;'},
    'This card delegates to a DMN decision. ',
    el('a', {href: '#', style: 'color:#0969DA;'}, 'Open the DMN cornerstone file'),
    ' for the decision table visualiser.',
  ));
  return root;
}

// ── Composite placeholder (call-tree is v1.5) ────────────────────
function renderComposite(card: AlgorithmCard): HTMLElement {
  const composed = card.implementation?.composite?.composed_of || [];
  const root = el('div', {style: 'padding:16px 20px;'});
  root.appendChild(el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, `composite · ${composed.length} step(s)`));
  composed.forEach((step, i) => {
    root.appendChild(el('div', {style: 'padding:8px 12px;border:1px solid #E5E2DC;border-radius:3px;margin-bottom:6px;'},
      el('div', {style: 'font-family:\"Source Code Pro\",monospace;font-size:12px;color:#3D3929;'}, `${i + 1}. ${step.id}`),
      step.description ? el('div', {style: 'font-size:11px;color:#5F5E5A;margin-top:2px;'}, step.description) : null,
    ));
  });
  return root;
}

// ── Polymorphic dispatch ─────────────────────────────────────────
function renderBody(card: AlgorithmCard): HTMLElement {
  const impl = card.implementation || {};
  if (impl.type === 'inline') {
    const lang = impl.inline?.language || impl.language;
    if (lang === 'regex') return renderInlineRegex(card);
    if (lang === 'feel') return renderInlineFeel(card);
    if (lang === 'dmn') return renderInlineDmnLinkOut(card);
    // rego, sql, wasm, other → fall back to source + sample browser
    return renderInlineFallback(card, lang || 'unknown');
  }
  if (impl.type === 'composite') return renderComposite(card);
  // 'external' or unknown → sample browser
  return renderExternalSampleBrowser(card);
}

function renderInlineFallback(card: AlgorithmCard, language: string): HTMLElement {
  const source = card.implementation?.inline?.source || '';
  const root = el('div', {style: 'padding:16px 20px;'});
  root.appendChild(el('div', {style: 'font-size:11px;color:#888780;text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px;'}, `inline · ${language}`));
  root.appendChild(el('pre', {style: 'background:#1F2328;color:#E8E5DA;padding:12px;border-radius:3px;font-family:\"Source Code Pro\",monospace;font-size:13px;overflow-x:auto;max-height:280px;'}, source));
  return root;
}

// ── Adapter entry point ──────────────────────────────────────────
export function createCardAdapter(canvas: HTMLElement, _properties: HTMLElement | null): DiagramAdapter {
  let currentCard: AlgorithmCard | null = null;

  return {
    async renderPreview(content: any): Promise<void> {
      clear(canvas);
      const text = typeof content === 'string' ? content : String(content);

      let card: AlgorithmCard;
      try {
        card = yaml.load(text) as AlgorithmCard;
      } catch (err) {
        canvas.appendChild(el('div', {style: 'padding:20px;color:#E24B4A;'}, `Failed to parse card YAML: ${(err as Error).message}`));
        return;
      }
      if (!card || typeof card !== 'object' || !card.id) {
        canvas.appendChild(el('div', {style: 'padding:20px;color:#E24B4A;'}, 'Not a valid UAPF algorithm card (missing id).'));
        return;
      }
      currentCard = card;

      // Compose: header + IO contract + polymorphic body
      const wrap = el('div', {class: 'uapf-card-viewer', style: 'max-width:980px;margin:0 auto;background:#FFFFFF;'});
      wrap.appendChild(renderHeader(card));
      wrap.appendChild(renderIOContract(card));
      wrap.appendChild(renderBody(card));

      // Footer: tests count + spec reference
      const footer = el('div', {style: 'padding:12px 20px;background:#F8F7F4;border-top:1px solid #E5E2DC;font-size:11px;color:#888780;display:flex;justify-content:space-between;'},
        el('span', {}, `${(card.tests || []).length} embedded test case${(card.tests || []).length === 1 ? '' : 's'}`),
        el('span', {}, 'UAPF v2.5.0 · chapter 13.16 viewer contract'),
      );
      wrap.appendChild(footer);

      canvas.appendChild(wrap);
    },

    // "Edit File" → redirect to Gitea's built-in editor. We get the file
    // path from window.location (the repo file view) and navigate to the
    // _edit/ URL. Done lazily here instead of from index.ts so we keep
    // the dispatch generic.
    async enterEdit(_content: any): Promise<void> {
      const loc = window.location.pathname;
      // /{owner}/{repo}/src/branch/{branch}/{path} → /{owner}/{repo}/_edit/{branch}/{path}
      const editUrl = loc.replace('/src/branch/', '/_edit/').replace('/src/commit/', '/_edit/');
      if (editUrl === loc) {
        // fallback: do nothing
        return;
      }
      window.location.href = editUrl;
    },

    destroy(): void {
      clear(canvas);
      currentCard = null;
    },
  };
}
