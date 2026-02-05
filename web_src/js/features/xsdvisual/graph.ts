import type {ComplexType, ElementDecl, GraphEdge, GraphModel, GraphNode, Occurs, Particle, SchemaDoc} from './types.ts';

const NODE_WIDTH = 200;
const NODE_HEIGHT = 64;
const COLUMN_GAP = 260;
const ROW_GAP = 110;

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

  const columnByKind: Record<GraphNode['kind'], number> = {
    schema: 0,
    element: 1,
    type: 2,
    group: 3,
  };
  const rowOffsets = new Map<number, number>();
  const nextRow = (col: number) => {
    const current = rowOffsets.get(col) ?? 0;
    rowOffsets.set(col, current + 1);
    return current;
  };

  nodes.forEach((node) => {
    const col = columnByKind[node.kind] ?? 0;
    const row = nextRow(col);
    node.bbox = {
      x: col * COLUMN_GAP,
      y: row * ROW_GAP,
      w: NODE_WIDTH,
      h: NODE_HEIGHT,
    };
  });

  return {nodes, edges, nodeById};
}
