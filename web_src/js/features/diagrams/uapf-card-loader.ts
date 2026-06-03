// UAPF Algorithm Card sidecar loader for bpmn-js custom renderer.
//
// Given an algorithm card id like "algo.semantic_document_analysis.pii_redactor",
// fetches the corresponding YAML file from the same repo's algorithms/ folder
// via the Gitea raw-file endpoint. The card id's terminal segment (after the
// last dot) is treated as the filename: algorithms/{terminal}.card.yaml.
//
// Cached per-page (per card id).

export interface CardMeta {
  id?: string;
  name?: string;
  version?: string;
  algorithm_kind?: string;
  determinism?: string;
  risk?: {
    aiActRiskClass?: string;
    humanOversight?: string;
  };
}

const cardCache = new Map<string, CardMeta | null>();

function repoBasePath(): string | null {
  // Gitea URLs look like /:owner/:repo/...
  // We want /:owner/:repo as the prefix for raw URLs.
  const m = window.location.pathname.match(/^\/([^/]+)\/([^/]+)/);
  if (!m) return null;
  return `/${m[1]}/${m[2]}`;
}

function currentBranch(): string {
  // /:owner/:repo/src/branch/:branch/...  or  /:owner/:repo/raw/branch/:branch/...
  const m = window.location.pathname.match(/\/(?:src|raw)\/(?:branch|commit)\/([^/]+)/);
  return m ? decodeURIComponent(m[1]) : 'main';
}

function cardIdToFilename(cardId: string): string {
  // "algo.semantic_document_analysis.pii_redactor" -> "pii_redactor.card.yaml"
  const segments = cardId.split('.');
  const terminal = segments[segments.length - 1];
  return `${terminal}.card.yaml`;
}

// Minimal YAML field-extractor: handles top-level "key: value" pairs.
// We don't need a full YAML parser — we only need a handful of scalars
// (version, name, algorithm_kind, determinism, risk.aiActRiskClass,
// risk.humanOversight). Cards are well-formed YAML authored by us.
function extractField(yaml: string, key: string): string | null {
  // Top-level: line begins with key followed by colon
  const re = new RegExp(`^${key}:\\s*(.+?)\\s*$`, 'm');
  const m = yaml.match(re);
  if (!m) return null;
  let v = m[1].trim();
  // strip surrounding quotes
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    v = v.slice(1, -1);
  }
  return v;
}

function extractNested(yaml: string, parent: string, child: string): string | null {
  // Look for `parent:` line followed by indented `child:` line
  const re = new RegExp(`^${parent}:\\s*\\n(?:^[ \\t]+.*\\n)*?^[ \\t]+${child}:\\s*(.+?)\\s*$`, 'm');
  const m = yaml.match(re);
  if (!m) return null;
  let v = m[1].trim();
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    v = v.slice(1, -1);
  }
  return v;
}

function parseCardMeta(yaml: string): CardMeta {
  const meta: CardMeta = {};
  const id = extractField(yaml, 'id'); if (id) meta.id = id;
  const name = extractField(yaml, 'name'); if (name) meta.name = name;
  const version = extractField(yaml, 'version'); if (version) meta.version = version;
  const kind = extractField(yaml, 'algorithm_kind'); if (kind) meta.algorithm_kind = kind;
  const det = extractField(yaml, 'determinism'); if (det) meta.determinism = det;

  const ai = extractNested(yaml, 'risk', 'aiActRiskClass');
  const ov = extractNested(yaml, 'risk', 'humanOversight');
  if (ai || ov) {
    meta.risk = {};
    if (ai) meta.risk.aiActRiskClass = ai;
    if (ov) meta.risk.humanOversight = ov;
  }
  return meta;
}

export async function loadCardSidecar(cardId: string): Promise<CardMeta | null> {
  if (cardCache.has(cardId)) return cardCache.get(cardId) ?? null;

  const repoBase = repoBasePath();
  if (!repoBase) {
    cardCache.set(cardId, null);
    return null;
  }
  const filename = cardIdToFilename(cardId);
  const branch = currentBranch();
  const url = `${repoBase}/raw/branch/${encodeURIComponent(branch)}/algorithms/${encodeURIComponent(filename)}`;

  try {
    const response = await fetch(url, {
      headers: {Accept: 'text/plain'},
      credentials: 'same-origin',
    });
    if (!response.ok) {
      cardCache.set(cardId, null);
      return null;
    }
    const text = await response.text();
    const meta = parseCardMeta(text);
    cardCache.set(cardId, meta);
    return meta;
  } catch {
    cardCache.set(cardId, null);
    return null;
  }
}

/**
 * Returns one of: 'green' | 'amber' | 'red' | 'unknown'
 * Mapping (per UAPF v2.4.0 chapter 13.10):
 *   - red:    risk.aiActRiskClass = high OR risk.humanOversight = mandatory
 *   - amber:  determinism in (stochastic, learned) OR risk.aiActRiskClass = limited with advisory oversight
 *   - green:  deterministic and risk class minimal/limited with non-mandatory oversight
 */
export function getRiskClass(meta: CardMeta): 'green' | 'amber' | 'red' | 'unknown' {
  if (!meta) return 'unknown';
  const aiRisk = meta.risk?.aiActRiskClass;
  const ov = meta.risk?.humanOversight;
  const det = meta.determinism || 'deterministic';

  if (aiRisk === 'high' || ov === 'mandatory') return 'red';
  if (det === 'stochastic' || det === 'learned') return 'amber';
  if (aiRisk === 'limited' && ov && ov !== 'none') return 'amber';
  return 'green';
}
