import type {GraphEdge, GraphModel, GraphNode} from './types.ts';

export interface RenderResult {
  setSelectedNode: (nodeId: string | null) => void;
  setSearchQuery: (query: string) => void;
  destroy: () => void;
}

function createSvgElement<K extends keyof SVGElementTagNameMap>(tag: K): SVGElementTagNameMap[K] {
  return document.createElementNS('http://www.w3.org/2000/svg', tag);
}

function getNodeCenter(node: GraphNode): {x: number; y: number} {
  const bbox = node.bbox ?? {x: 0, y: 0, w: 0, h: 0};
  return {x: bbox.x + bbox.w / 2, y: bbox.y + bbox.h / 2};
}

function renderEdge(edge: GraphEdge, nodes: Map<string, GraphNode>): SVGElement {
  const group = createSvgElement('g');
  const fromNode = nodes.get(edge.from);
  const toNode = nodes.get(edge.to);
  if (!fromNode || !toNode) return group;

  const from = getNodeCenter(fromNode);
  const to = getNodeCenter(toNode);

  const path = createSvgElement('path');
  const pts = edge.points?.length ? edge.points : [from, to];
  const d = pts
    .map((point, idx) => `${idx === 0 ? 'M' : 'L'} ${point.x} ${point.y}`)
    .join(' ');
  path.setAttribute('d', d);
  path.setAttribute('fill', 'none');
  path.setAttribute('stroke', 'var(--xsd-edge-color)');
  path.setAttribute('stroke-width', '1.25');
  path.setAttribute('stroke-linejoin', 'round');
  path.setAttribute('stroke-linecap', 'round');
  path.setAttribute('marker-end', 'url(#xsd-visual-arrow)');
  group.append(path);

  if (edge.label && pts.length >= 2) {
    const mid = pts[Math.floor(pts.length / 2)];
    const label = createSvgElement('text');
    label.textContent = edge.label;
    label.setAttribute('x', String(mid.x + 6));
    label.setAttribute('y', String(mid.y - 6));
    label.setAttribute('font-size', '10');
    label.setAttribute('fill', 'var(--xsd-label-color)');
    group.append(label);
  }

  return group;
}

function renderNode(node: GraphNode, onSelect: (id: string) => void): SVGGElement {
  const group = createSvgElement('g');
  group.setAttribute('data-node-id', node.id);
  group.setAttribute('cursor', 'pointer');
  group.classList.add('xsd-node', `xsd-node-${node.kind}`);

  const bbox = node.bbox ?? {x: 0, y: 0, w: 180, h: 96};
  const rect = createSvgElement('rect');
  rect.setAttribute('x', String(bbox.x));
  rect.setAttribute('y', String(bbox.y));
  rect.setAttribute('width', String(bbox.w));
  rect.setAttribute('height', String(bbox.h));
  rect.setAttribute('rx', '8');
  rect.setAttribute('filter', 'url(#xsd-card-shadow)');

  const header = createSvgElement('rect');
  header.setAttribute('x', String(bbox.x));
  header.setAttribute('y', String(bbox.y));
  header.setAttribute('width', String(bbox.w));
  header.setAttribute('height', '26');
  header.setAttribute('rx', '8');
  header.setAttribute('ry', '8');
  header.classList.add('xsd-node-header');

  const divider = createSvgElement('line');
  divider.setAttribute('x1', String(bbox.x));
  divider.setAttribute('y1', String(bbox.y + 26));
  divider.setAttribute('x2', String(bbox.x + bbox.w));
  divider.setAttribute('y2', String(bbox.y + 26));
  divider.classList.add('xsd-node-divider');

  const icon = createSvgElement('text');
  icon.textContent = node.kind === 'schema' ? '⎈' : node.kind === 'type' ? '⌗' : '▦';
  icon.setAttribute('x', String(bbox.x + 10));
  icon.setAttribute('y', String(bbox.y + 18));
  icon.classList.add('xsd-node-icon');

  const title = createSvgElement('text');
  title.textContent = node.label;
  title.setAttribute('x', String(bbox.x + 28));
  title.setAttribute('y', String(bbox.y + 18));
  title.classList.add('xsd-node-title');

  const rows: string[] = [];
  if (node.meta.type) rows.push(`type: ${node.meta.type}`);
  if (node.meta.base) rows.push(`base: ${node.meta.base}`);
  if (node.meta.occurs) rows.push(`occurs: ${node.meta.occurs}`);
  if (node.meta.attributes) {
    const attrs = node.meta.attributes
      .split('|')
      .map((value) => value.trim())
      .filter(Boolean);
    if (attrs.length) {
      rows.push(`attributes: ${attrs.slice(0, 3).join(', ')}${attrs.length > 3 ? ', …' : ''}`);
    }
  }

  group.append(rect, header, divider, icon, title);

  let rowY = bbox.y + 42;
  for (const row of rows.slice(0, 4)) {
    const text = createSvgElement('text');
    text.textContent = row;
    text.setAttribute('x', String(bbox.x + 12));
    text.setAttribute('y', String(rowY));
    text.classList.add('xsd-node-row');
    group.append(text);
    rowY += 14;
  }

  group.addEventListener('click', (event) => {
    event.stopPropagation();
    onSelect(node.id);
  });

  group.addEventListener('mouseenter', () => {
    rect.setAttribute('stroke-width', '2');
  });
  group.addEventListener('mouseleave', () => {
    if (rect.getAttribute('data-selected') === 'true') return;
    rect.setAttribute('stroke-width', '1');
  });

  return group;
}

export function renderGraph(
  mount: HTMLElement,
  model: GraphModel,
  opts: {onSelect: (nodeId: string | null) => void},
): RenderResult {
  mount.replaceChildren();

  const svg = createSvgElement('svg');
  svg.setAttribute('width', '100%');
  svg.setAttribute('height', '100%');
  svg.classList.add('xsd-visual-canvas');
  svg.style.display = 'block';
  svg.style.minHeight = '420px';

  const defs = createSvgElement('defs');
  const marker = createSvgElement('marker');
  marker.setAttribute('id', 'xsd-visual-arrow');
  marker.setAttribute('markerWidth', '10');
  marker.setAttribute('markerHeight', '10');
  marker.setAttribute('refX', '6');
  marker.setAttribute('refY', '3');
  marker.setAttribute('orient', 'auto');
  const markerPath = createSvgElement('path');
  markerPath.setAttribute('d', 'M0,0 L0,6 L6,3 z');
  markerPath.setAttribute('fill', 'var(--xsd-edge-color)');
  marker.append(markerPath);
  defs.append(marker);

  const filter = createSvgElement('filter');
  filter.setAttribute('id', 'xsd-card-shadow');
  filter.setAttribute('x', '-20%');
  filter.setAttribute('y', '-20%');
  filter.setAttribute('width', '140%');
  filter.setAttribute('height', '140%');
  const feDrop = createSvgElement('feDropShadow');
  feDrop.setAttribute('dx', '0');
  feDrop.setAttribute('dy', '1');
  feDrop.setAttribute('stdDeviation', '1.2');
  feDrop.setAttribute('flood-opacity', '0.18');
  filter.append(feDrop);
  defs.append(filter);
  svg.append(defs);

  const viewport = createSvgElement('g');
  svg.append(viewport);

  const edgeLayer = createSvgElement('g');
  const nodeLayer = createSvgElement('g');
  viewport.append(edgeLayer, nodeLayer);

  model.edges.forEach((edge) => {
    edgeLayer.append(renderEdge(edge, model.nodeById));
  });

  const nodeEls = new Map<string, SVGRectElement>();

  model.nodes.forEach((node) => {
    const nodeGroup = renderNode(node, (id) => opts.onSelect(id));
    const rect = nodeGroup.querySelector('rect');
    if (rect) nodeEls.set(node.id, rect);
    nodeLayer.append(nodeGroup);
  });

  let selectedId: string | null = null;

  const setSelectedNode = (nodeId: string | null) => {
    if (selectedId && nodeEls.has(selectedId)) {
      const previous = nodeEls.get(selectedId);
      if (previous) {
        previous.setAttribute('data-selected', 'false');
        previous.setAttribute('stroke-width', '1');
      }
    }
    selectedId = nodeId;
    if (selectedId && nodeEls.has(selectedId)) {
      const current = nodeEls.get(selectedId);
      if (current) {
        current.setAttribute('data-selected', 'true');
        current.setAttribute('stroke-width', '2');
      }
    }
  };

  const setSearchQuery = (query: string) => {
    const q = query.trim().toLowerCase();
    model.nodes.forEach((node) => {
      const el = nodeLayer.querySelector<SVGGElement>(`g[data-node-id="${node.id}"]`);
      if (!el) return;
      if (!q) {
        el.style.opacity = '1';
        return;
      }
      const matches = node.label.toLowerCase().includes(q);
      el.style.opacity = matches ? '1' : '0.2';
    });
  };

  let isPanning = false;
  let panStart = {x: 0, y: 0};
  let translate = {x: 24, y: 24};
  let scale = 1;

  const applyTransform = () => {
    viewport.setAttribute('transform', `translate(${translate.x}, ${translate.y}) scale(${scale})`);
  };
  applyTransform();

  const onMouseDown = (event: MouseEvent) => {
    if (event.button !== 0) return;
    const target = event.target as HTMLElement;
    if (target.closest('g[data-node-id]')) return;
    isPanning = true;
    panStart = {x: event.clientX - translate.x, y: event.clientY - translate.y};
  };

  const onMouseMove = (event: MouseEvent) => {
    if (!isPanning) return;
    translate = {x: event.clientX - panStart.x, y: event.clientY - panStart.y};
    applyTransform();
  };

  const onMouseUp = () => {
    isPanning = false;
  };

  const onWheel = (event: WheelEvent) => {
    event.preventDefault();
    const delta = event.deltaY < 0 ? 1.1 : 0.9;
    scale = Math.min(2.5, Math.max(0.3, scale * delta));
    applyTransform();
  };

  const onCanvasClick = () => {
    opts.onSelect(null);
  };

  svg.addEventListener('mousedown', onMouseDown);
  svg.addEventListener('mousemove', onMouseMove);
  svg.addEventListener('mouseup', onMouseUp);
  svg.addEventListener('mouseleave', onMouseUp);
  svg.addEventListener('wheel', onWheel, {passive: false});
  svg.addEventListener('click', onCanvasClick);

  mount.append(svg);

  return {
    setSelectedNode,
    setSearchQuery,
    destroy: () => {
      svg.removeEventListener('mousedown', onMouseDown);
      svg.removeEventListener('mousemove', onMouseMove);
      svg.removeEventListener('mouseup', onMouseUp);
      svg.removeEventListener('mouseleave', onMouseUp);
      svg.removeEventListener('wheel', onWheel);
      svg.removeEventListener('click', onCanvasClick);
    },
  };
}
