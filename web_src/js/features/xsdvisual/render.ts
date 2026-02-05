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

function buildMetaLine(node: GraphNode): string {
  const parts: string[] = [];
  if (node.meta.type) parts.push(`type ${node.meta.type}`);
  if (node.meta.base) parts.push(`base ${node.meta.base}`);
  if (node.meta.occurs) parts.push(node.meta.occurs);
  if (node.meta.targetNamespace) parts.push(node.meta.targetNamespace);
  return parts.join(' • ');
}

function renderEdge(edge: GraphEdge, nodes: Map<string, GraphNode>): SVGElement {
  const group = createSvgElement('g');
  const fromNode = nodes.get(edge.from);
  const toNode = nodes.get(edge.to);
  if (!fromNode || !toNode) return group;

  const from = getNodeCenter(fromNode);
  const to = getNodeCenter(toNode);

  const path = createSvgElement('path');
  const midX = (from.x + to.x) / 2;
  const d = `M ${from.x} ${from.y} L ${midX} ${from.y} L ${midX} ${to.y} L ${to.x} ${to.y}`;
  path.setAttribute('d', d);
  path.setAttribute('fill', 'none');
  path.setAttribute('stroke', 'currentColor');
  path.setAttribute('stroke-width', '1');
  path.setAttribute('marker-end', 'url(#xsd-visual-arrow)');
  group.append(path);

  if (edge.label) {
    const label = createSvgElement('text');
    label.textContent = edge.label;
    label.setAttribute('x', String(midX + 4));
    label.setAttribute('y', String((from.y + to.y) / 2 - 4));
    label.setAttribute('font-size', '10');
    label.setAttribute('fill', 'currentColor');
    group.append(label);
  }

  return group;
}

function renderNode(node: GraphNode, onSelect: (id: string) => void): SVGGElement {
  const group = createSvgElement('g');
  group.setAttribute('data-node-id', node.id);
  group.setAttribute('cursor', 'pointer');

  const bbox = node.bbox ?? {x: 0, y: 0, w: 180, h: 60};
  const rect = createSvgElement('rect');
  rect.setAttribute('x', String(bbox.x));
  rect.setAttribute('y', String(bbox.y));
  rect.setAttribute('width', String(bbox.w));
  rect.setAttribute('height', String(bbox.h));
  rect.setAttribute('rx', '8');
  rect.setAttribute('fill', 'none');
  rect.setAttribute('stroke', 'currentColor');
  rect.setAttribute('stroke-width', '1');

  const title = createSvgElement('text');
  title.textContent = node.label;
  title.setAttribute('x', String(bbox.x + 12));
  title.setAttribute('y', String(bbox.y + 22));
  title.setAttribute('font-size', '12');
  title.setAttribute('fill', 'currentColor');

  const metaLine = buildMetaLine(node);
  if (metaLine) {
    const meta = createSvgElement('text');
    meta.textContent = metaLine;
    meta.setAttribute('x', String(bbox.x + 12));
    meta.setAttribute('y', String(bbox.y + 40));
    meta.setAttribute('font-size', '10');
    meta.setAttribute('fill', 'currentColor');
    group.append(meta);
  }

  group.append(rect, title);

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
  markerPath.setAttribute('fill', 'currentColor');
  marker.append(markerPath);
  defs.append(marker);
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
