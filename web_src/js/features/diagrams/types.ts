export interface DiagramPayload {
  type: string;
  format: 'xml' | 'json' | string;
  path: string;
  branch: string;
  lastCommit: string;
  repoLink: string;
  content: string;
  encoding?: string;
  editable?: boolean;
  sourcePath?: string;
}

export interface DiagramAdapter {
  renderPreview(content: any): Promise<void>;
  enterEdit?: (content: any) => Promise<void>;
  save?: () => Promise<string>;
  setChangeHandler?: (handler: () => void) => void;
  destroy?: () => void;
}
