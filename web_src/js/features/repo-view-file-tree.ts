import {createApp, type App} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import ViewFileTree from '../components/ViewFileTree.vue';
import ChatPanel from '../components/ChatPanel.vue';
import {registerGlobalEventFunc} from '../modules/observer.ts';

const {appSubUrl} = window.config;

function isUserSignedIn() {
  return Boolean(document.querySelector('#navbar .user-menu'));
}

async function toggleSidebar(btn: HTMLElement) {
  const elToggleShow = document.querySelector('.repo-view-file-tree-toggle[data-toggle-action="show"]')!;
  const elFileTreeContainer = document.querySelector('.repo-view-file-tree-container')!;
  const shouldShow = btn.getAttribute('data-toggle-action') === 'show';
  toggleElem(elFileTreeContainer, shouldShow);
  toggleElem(elToggleShow, !shouldShow);

  // FIXME: need to remove "full height" style from parent element

  if (!isUserSignedIn()) return;
  await POST(`${appSubUrl}/user/settings/update_preferences`, {
    data: {codeViewShowFileTree: shouldShow},
  });
}

let chatApp: App | null = null;
let chatMountEl: HTMLElement | null = null;

function openChatPanel(detail: {repoLink: string; agentFile: string; agentName: string}) {
  // Close existing chat panel if any
  closeChatPanel();

  const repoViewContent = document.querySelector('.repo-view-content');
  if (!repoViewContent) return;

  // Create mount point
  chatMountEl = document.createElement('div');
  chatMountEl.className = 'chat-panel-container';
  repoViewContent.innerHTML = '';
  repoViewContent.appendChild(chatMountEl);

  chatApp = createApp(ChatPanel, {
    repoLink: detail.repoLink,
    agentFile: detail.agentFile,
    agentName: detail.agentName,
    onClose: () => closeChatPanel(),
  });
  chatApp.mount(chatMountEl);
}

function closeChatPanel() {
  if (chatApp) {
    chatApp.unmount();
    chatApp = null;
  }
  if (chatMountEl) {
    chatMountEl.remove();
    chatMountEl = null;
  }
}

export async function initRepoViewFileTree() {
  const sidebar = document.querySelector<HTMLElement>('.repo-view-file-tree-container');
  const repoViewContent = document.querySelector('.repo-view-content');
  if (!sidebar || !repoViewContent) return;

  registerGlobalEventFunc('click', 'onRepoViewFileTreeToggle', toggleSidebar);

  const fileTree = sidebar.querySelector('#view-file-tree')!;
  createApp(ViewFileTree, {
    repoLink: fileTree.getAttribute('data-repo-link'),
    treePath: fileTree.getAttribute('data-tree-path'),
    currentRefNameSubURL: fileTree.getAttribute('data-current-ref-name-sub-url'),
  }).mount(fileTree);

  // Listen for chat agent open events from the file tree
  window.addEventListener('open-chat-agent', ((e: CustomEvent) => {
    openChatPanel(e.detail);
  }) as EventListener);
}
