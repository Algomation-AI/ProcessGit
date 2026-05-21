// UAPF Algorithm Card visual decoration for bpmn-js, via the public Overlays API.
//
// For every bpmn:serviceTask / bpmn:task carrying the v2.4.0
// uapf24:algorithmCardRef attribute, attaches an HTML overlay with:
//   - a custom "algorithm card" icon (three stacked rectangles + ƒ)
//   - the card id (truncated if long)
//   - a colored risk-class dot (green / amber / red)
// Card metadata is loaded asynchronously from algorithms/{id}.card.yaml
// in the same repo. The risk dot starts neutral and updates once the
// card sidecar resolves.
//
// This avoids adding bpmn-js BaseRenderer subclasses — it sits entirely
// on the public Overlays API.

import {getRiskClass, loadCardSidecar, type CardMeta} from './uapf-card-loader.ts';

const UAPF_NS_V24 = 'https://uapf.dev/bpmn/v2.4';

function readAlgorithmCardRef(businessObject: any): string | null {
  if (!businessObject) return null;
  const attrs = businessObject.$attrs || {};
  const candidates = [
    attrs['uapf24:algorithmCardRef'],
    attrs['uapf:algorithmCardRef'],
    attrs[`{${UAPF_NS_V24}}algorithmCardRef`],
    attrs['algorithmCardRef'],
  ];
  for (const v of candidates) {
    if (typeof v === 'string' && v.length > 0) return v;
  }
  return null;
}

const RISK_DOT_COLOR: Record<string, string> = {
  green: '#1D9E75',
  amber: '#EF9F27',
  red: '#E24B4A',
  unknown: '#888780',
};

function buildOverlayHtml(cardRef: string, meta: CardMeta | null): HTMLElement {
  const root = document.createElement('div');
  root.className = 'uapf-algo-overlay';
  root.style.cssText = [
    'position: absolute',
    'top: 0',
    'left: 0',
    'width: 100%',
    'height: 100%',
    'pointer-events: none',
    'font-family: Arial, sans-serif',
    'font-size: 10px',
    'color: #5F5E5A',
  ].join('; ');

  // Algorithm icon — three stacked cards with ƒ — placed top-left over the
  // default serviceTask gear position.
  const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  icon.setAttribute('width', '34');
  icon.setAttribute('height', '24');
  icon.setAttribute('viewBox', '0 0 34 24');
  icon.style.cssText = 'position:absolute;top:4px;left:4px;background:#FFFFFF;border-radius:2px;';
  icon.innerHTML = [
    '<rect x="8" y="2"  width="22" height="14" rx="2" fill="#FFFFFF" stroke="#1F2328" stroke-width="1"/>',
    '<rect x="5" y="5"  width="22" height="14" rx="2" fill="#FFFFFF" stroke="#1F2328" stroke-width="1"/>',
    '<rect x="2" y="8"  width="22" height="14" rx="2" fill="#FFFFFF" stroke="#1F2328" stroke-width="1"/>',
    '<text x="13" y="19" text-anchor="middle" font-family="serif" font-size="11" font-style="italic" fill="#1F2328">ƒ</text>',
  ].join('');
  root.appendChild(icon);

  // Risk dot top-right
  const dot = document.createElement('div');
  const riskClass = meta ? getRiskClass(meta) : 'unknown';
  dot.className = `uapf-risk-dot uapf-risk-${riskClass}`;
  dot.style.cssText = [
    'position:absolute',
    'top:6px',
    'right:6px',
    'width:10px',
    'height:10px',
    'border-radius:50%',
    `background:${RISK_DOT_COLOR[riskClass] || RISK_DOT_COLOR.unknown}`,
    'border:1px solid #FFFFFF',
  ].join('; ');
  dot.title = `Algorithm card: ${cardRef}` + (meta ? ` (risk: ${riskClass})` : '');
  root.appendChild(dot);

  // Two-line label at the bottom of the task: card id, then metadata strip
  const labelBox = document.createElement('div');
  labelBox.style.cssText = [
    'position:absolute',
    'bottom:2px',
    'left:0',
    'right:0',
    'text-align:center',
    'padding:0 4px',
    'line-height:1.2',
    'background:linear-gradient(to top, #FFFFFF 60%, rgba(255,255,255,0))',
  ].join('; ');

  const idLine = document.createElement('div');
  idLine.style.cssText = 'font-size:9px;color:#5F5E5A;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
  idLine.textContent = cardRef;
  idLine.title = cardRef;
  labelBox.appendChild(idLine);

  if (meta) {
    const metaLine = document.createElement('div');
    metaLine.style.cssText = 'font-size:9px;color:#888780;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
    const parts = [
      meta.version ? 'v' + meta.version : null,
      meta.algorithm_kind || null,
      meta.determinism || 'deterministic',
    ].filter(Boolean);
    metaLine.textContent = parts.join(' · ');
    labelBox.appendChild(metaLine);
  }

  root.appendChild(labelBox);
  return root;
}

/**
 * Walks all shapes in the diagram. For each serviceTask/task with an
 * algorithmCardRef, adds an Overlay. The overlay is created immediately
 * with neutral metadata and updated when the card sidecar loads.
 */
export function attachUapfAlgorithmOverlays(viewerOrModeler: any): void {
  if (!viewerOrModeler) return;
  let overlays: any;
  let elementRegistry: any;
  try {
    overlays = viewerOrModeler.get('overlays');
    elementRegistry = viewerOrModeler.get('elementRegistry');
  } catch (err) {
    console.debug('[uapf-overlay] bpmn-js modules unavailable', err);
    return;
  }
  if (!overlays || !elementRegistry) return;

  const elements: any[] = elementRegistry.getAll();
  for (const element of elements) {
    const bo = element.businessObject;
    if (!bo) continue;
    const type = bo.$type;
    if (type !== 'bpmn:ServiceTask' && type !== 'bpmn:Task') continue;
    const cardRef = readAlgorithmCardRef(bo);
    if (!cardRef) continue;

    // Add an initial overlay with neutral risk
    const html = buildOverlayHtml(cardRef, null);
    const overlayId = overlays.add(element.id, 'uapf-algo-card', {
      position: {top: 0, left: 0},
      html,
    });

    // Try to fetch the card sidecar; on resolve, swap the overlay HTML
    loadCardSidecar(cardRef).then((meta) => {
      if (!meta) return;
      overlays.remove(overlayId);
      const newHtml = buildOverlayHtml(cardRef, meta);
      overlays.add(element.id, 'uapf-algo-card', {
        position: {top: 0, left: 0},
        html: newHtml,
      });
    }).catch((err) => {
      console.debug('[uapf-overlay] card sidecar fetch failed', cardRef, err);
    });
  }
}
