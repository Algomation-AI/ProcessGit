import dagre from 'dagre';
import type {ComplexType, ElementDecl, GraphEdge, GraphModel, GraphNode, Occurs, Particle, SchemaDoc} from './types.ts';

const NODE_WIDTH = 200;
const NODE_HEIGHT = 96;

export function elementNodeId(name: string, parent?: string): string {
  return parent ? `element:${parent}/${name}` : `element:${name}`;
}

export function typeNodeId(name: string): string {
  return `type:${name}`;
}

function formatOccurs(minOccurs?: number, maxOccurs?: Occurs): string {
  const min = minOccurs ?? 1;
  const max = maxOccurs ?? 1;
  return `${min}..${max}`;
}

function addElementNode(nodes: GraphNode[], nodeById: Map<string, GraphNode>, element: ElementDecl, parent?: string) {
  const id = elementNodeId(element.name, parent);
  const meta: Record<string, string> = {};
  if (element.type) meta.type = element.type;
  const occurs = formatOccurs(element.minOccurs, element.maxOccurs);
  meta.occurs = occurs;
  if (element.annotation) meta.documentation = element.annotation;
  if (element.attributes?.length) {
    meta.attributes = element.attributes
      .map((attr) => attr.name ?? attr.ref ?? '')
      .filter(Boolean)
      .join('|');
  }
  const node: GraphNode = {
    id,
    kind: 'element',
    label: element.name,
    meta,
  };
  nodes.push(node);
  nodeById.set(id, node);
  return id;
}

function addTypeNode(nodes: GraphNode[], nodeById: Map<string, GraphNode>, type: ComplexType) {
  const id = typeNodeId(type.name);
  const meta: Record<string, string> = {};
  if (type.base) meta.base = type.base;
  if (type.annotation) meta.documentation = type.annotation;
  if (type.attributes?.length) {
    meta.attributes = type.attributes
      .map((attr) => attr.name ?? attr.ref ?? '')
      .filter(Boolean)
      .join('|');
  }
  const node: GraphNode = {
    id,
    kind: 'type',
    label: type.name,
    meta,
  };
  nodes.push(node);
  nodeById.set(id, node);
  return id;
}

function addParticleNodes(
  particles: Particle[] | undefined,
  nodes: GraphNode[],
  nodeById: Map<string, GraphNode>,
  edges: GraphEdge[],
  parentId: string,
  parentName: string,
) {
  if (!particles) return;
  for (const particle of particles) {
    if (particle.kind === 'elementInline' && particle.element) {
      const childId = addElementNode(nodes, nodeById, particle.element, parentName);
      edges.push({
        from: parentId,
        to: childId,
        kind: 'contains',
        label: formatOccurs(particle.element.minOccurs, particle.element.maxOccurs),
      });
      if (particle.element.type) {
        edges.push({
          from: childId,
          to: typeNodeId(particle.element.type),
          kind: 'ref',
          label: 'type',
        });
      }
      continue;
    }
    if (particle.kind === 'elementRef' && particle.ref) {
      const refId = elementNodeId(particle.ref);
      edges.push({
        from: parentId,
        to: refId,
        kind: 'ref',
        label: formatOccurs(particle.minOccurs, particle.maxOccurs),
      });
      continue;
    }
  }
}

export function buildGraph(doc: SchemaDoc): GraphModel {
  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];
  const nodeById = new Map<string, GraphNode>();

  const schemaNode: GraphNode = {
    id: 'schema',
    kind: 'schema',
    label: 'schema',
    meta: doc.targetNamespace ? {targetNamespace: doc.targetNamespace} : {},
  };
  nodes.push(schemaNode);
  nodeById.set(schemaNode.id, schemaNode);

  for (const type of doc.types) {
    const typeId = addTypeNode(nodes, nodeById, type);
    edges.push({from: 'schema', to: typeId, kind: 'contains'});
    if (type.base) {
      edges.push({from: typeId, to: typeNodeId(type.base), kind: 'extends', label: 'extends'});
    }
    addParticleNodes(type.sequence ?? type.choice, nodes, nodeById, edges, typeNodeId(type.name), type.name);
  }

  for (const element of doc.elements) {
    const elemId = addElementNode(nodes, nodeById, element);
    edges.push({from: 'schema', to: elemId, kind: 'contains'});
    if (element.type) {
      edges.push({from: elemId, to: typeNodeId(element.type), kind: 'ref', label: 'type'});
    }
    if (element.children) {
      addParticleNodes(element.children, nodes, nodeById, edges, elementNodeId(element.name), element.name);
    }
  }

  const g = new dagre.graphlib.Graph();
  g.setGraph({
    rankdir: 'LR',
    nodesep: 60,
    ranksep: 140,
    marginx: 20,
    marginy: 20,
  });
  g.setDefaultEdgeLabel(() => ({}));

  nodes.forEach((node) => {
    g.setNode(node.id, {width: NODE_WIDTH, height: NODE_HEIGHT});
  });

  function edgeKey(from: string, to: string) {
    return `${from}=>${to}`;
  }

  const edgeByKey = new Map<string, GraphEdge>();

  edges.forEach((edge) => {
    const key = edgeKey(edge.from, edge.to);
    edgeByKey.set(key, edge);
    g.setEdge(edge.from, edge.to, {}, key);
  });

  dagre.layout(g);

  edgeByKey.forEach((edge, key) => {
    const layoutEdge = g.edge(edge.from, edge.to, key) as {points?: Array<{x: number; y: number}>} | undefined;
    if (!layoutEdge?.points?.length) return;
    edge.points = layoutEdge.points.map((point) => ({x: point.x, y: point.y}));
  });

  nodes.forEach((node) => {
    const layout = g.node(node.id) as {x: number; y: number; width: number; height: number} | undefined;
    if (!layout) return;
    node.bbox = {
      x: layout.x - layout.width / 2,
      y: layout.y - layout.height / 2,
      w: layout.width,
      h: layout.height,
    };
  });

  return {nodes, edges, nodeById};
}
