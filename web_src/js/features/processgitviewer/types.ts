export type ProcessGitViewerPayload = {
  id: string;
  type: 'html';
  repoLink: string;
  branch: string;
  ref: string;
  path: string;
  dir: string;
  lastCommit: string;
  entryRawUrl: string;
  targets: Record<string, string>;
  editAllow: string[];
  apiUrl: string;
};
