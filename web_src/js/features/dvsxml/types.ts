export interface DVSXMLPayload {
  type: string;
  path: string;
  ref: string;
  repoLink: string;
  rawUrl?: string;
  apiUrl: string;
  namespace?: string;
  schemaLocation?: string;
  meta?: Record<string, string>;
}

export interface ClassificationCase {
  uid?: string;
  index?: string;
  title?: string;
  description?: string;
  retention?: string;
  responsible?: string;
  unknownAttributes: Record<string, string>;
  unknownElements: string[];
}

export interface ClassificationGroup {
  uid?: string;
  index?: string;
  title?: string;
  description?: string;
  retention?: string;
  responsible?: string;
  cases: ClassificationCase[];
  unknownAttributes: Record<string, string>;
  unknownElements: string[];
}

export interface ClassificationSchemeModel {
  type: 'dvs.classification-scheme';
  schemeId?: string;
  version?: string;
  generatedAt?: string;
  organization?: {id?: string; name?: string};
  period?: {start?: string; end?: string};
  namespace?: string;
  schemaLocation?: string;
  meta?: Record<string, string>;
  groups: ClassificationGroup[];
  unknownAttributes: Record<string, string>;
  unknownElements: string[];
}

export interface DocumentMetadataEntry {
  uid?: string;
  title?: string;
  id?: string;
  status?: string;
  category?: string;
  exportedAt?: string;
  sourceSystem?: string;
  fields: Record<string, string>;
  attributes: Record<string, string>;
  unknownElements: string[];
}

export interface DocumentMetadataModel {
  type: 'dvs.document-metadata';
  exportedAt?: string;
  sourceSystem?: string;
  namespace?: string;
  schemaLocation?: string;
  meta?: Record<string, string>;
  documents: DocumentMetadataEntry[];
  unknownAttributes: Record<string, string>;
  unknownElements: string[];
}
