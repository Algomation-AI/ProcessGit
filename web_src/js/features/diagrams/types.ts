export interface DiagramPayload {
  type: string;
  format: 'xml' | 'json' | string;
  path: string;
  branch: string;
  lastCommit: string;
  repoLink: string;
  content: string;
  contentB64?: string;
  encoding?: string;
  editable?: boolean;
  sourcePath?: string;
}

export interface RawDiagramPayload extends Partial<DiagramPayload> {
  Type?: string;
  Format?: string;
  Content?: string;
  Encoding?: string;
  contentB64?: string;
}

export interface DiagramAdapter {
  renderPreview(content: any): Promise<void>;
  enterEdit?: (content: any) => Promise<void>;
  save?: () => Promise<string>;
  setChangeHandler?: (handler: () => void) => void;
  destroy?: () => void;
}
