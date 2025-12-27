import cytoscape from 'cytoscape';
import type {DiagramAdapter} from './types.ts';

interface NGraphNode {
  id: string;
  label?: string;
  type?: string;
}

interface NGraphEdge {
  source: string;
  target: string;
  label?: string;
}

interface NGraphData {
  graph?: {
    nodes?: NGraphNode[];
    edges?: NGraphEdge[];
  };
  nodes?: NGraphNode[];
  edges?: NGraphEdge[];
}

function normalizeGraph(data: any): {nodes: NGraphNode[]; edges: NGraphEdge[]} | null {
  const source: NGraphData = data?.graph ?? data ?? {};
  if (!source.nodes || !source.edges) return null;

  const nodes: NGraphNode[] = Array.isArray(source.nodes) ? source.nodes : [];
  const edges: NGraphEdge[] = Array.isArray(source.edges) ? source.edges : [];

  const normalizedNodes = nodes
    .map((node) => ({...node, id: `${node.id ?? node.label ?? ''}`}))
    .filter((node) => Boolean(node.id));
  const normalizedEdges = edges
    .map((edge) => ({
      ...edge,
      source: `${edge.source ?? ''}`,
      target: `${edge.target ?? ''}`,
    }))
    .filter((edge) => Boolean(edge.source) && Boolean(edge.target));

  if (!normalizedNodes.length || !normalizedEdges.length) return null;
  return {nodes: normalizedNodes, edges: normalizedEdges};
}

export function createNGraphAdapter(canvas: HTMLElement): DiagramAdapter {
  let cy: cytoscape.Core | null = null;

  return {
    async renderPreview(data: any) {
      const graph = normalizeGraph(data);
      if (!graph) throw new Error('Graph data is missing nodes or edges');

      cy?.destroy();
      canvas.innerHTML = '';
      cy = cytoscape({
        container: canvas,
        elements: [
          ...graph.nodes.map((node) => ({data: {id: node.id, label: node.label ?? node.id, type: node.type ?? ''}})),
          ...graph.edges.map((edge) => ({data: {source: edge.source, target: edge.target, label: edge.label ?? ''}})),
        ],
        layout: {name: 'breadthfirst', directed: true, padding: 30, spacingFactor: 1.2},
        style: [
          {
            selector: 'node',
            style: {
              label: 'data(label)',
              'text-valign': 'center',
              'text-halign': 'center',
              'font-size': 12,
              'background-color': '#3c6db0',
              color: '#fff',
              'border-width': 1,
              'border-color': '#1f4f85',
            },
          },
          {
            selector: 'edge',
            style: {
              width: 2,
              label: 'data(label)',
              'curve-style': 'bezier',
              'target-arrow-shape': 'triangle',
              'line-color': '#999',
              'target-arrow-color': '#999',
              'font-size': 11,
              'text-background-color': '#fff',
              'text-background-opacity': 0.7,
              'text-background-padding': 2,
            },
          },
        ],
      });

      cy.resize();
      cy.fit(undefined, 30);
    },

    destroy() {
      cy?.destroy();
      cy = null;
    },
  };
}
