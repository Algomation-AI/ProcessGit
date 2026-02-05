export type Occurs = number | 'unbounded';

export interface Annotation {
  documentation?: string;
}

export interface ElementDecl {
  name: string;
  type?: string;
  minOccurs?: number;
  maxOccurs?: Occurs;
  annotation?: string;
  children?: Particle[];
}

export interface ComplexType {
  name: string;
  base?: string;
  sequence?: Particle[];
  choice?: Particle[];
  annotation?: string;
}

export interface SimpleType {
  name: string;
  annotation?: string;
}

export interface Particle {
  kind: 'elementRef' | 'elementInline' | 'group' | 'any';
  ref?: string;
  element?: ElementDecl;
  minOccurs?: number;
  maxOccurs?: Occurs;
}

export interface SchemaDoc {
  targetNamespace?: string;
  elements: ElementDecl[];
  types: ComplexType[];
  simpleTypes: SimpleType[];
  annotations: Annotation[];
  includes: string[];
  imports: {namespace?: string; schemaLocation?: string}[];
}

export interface GraphNode {
  id: string;
  kind: 'schema' | 'element' | 'type' | 'group';
  label: string;
  meta: Record<string, string>;
  bbox?: {x: number; y: number; w: number; h: number};
}

export interface GraphEdge {
  from: string;
  to: string;
  label?: string;
  kind: 'contains' | 'extends' | 'ref';
}

export interface GraphModel {
  nodes: GraphNode[];
  edges: GraphEdge[];
  nodeById: Map<string, GraphNode>;
}

export interface ParsedXsd {
  doc: SchemaDoc;
  warnings: string[];
}
