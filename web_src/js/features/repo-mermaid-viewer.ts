import {registerGlobalInitFunc} from '../modules/observer.ts';
import {isDarkTheme} from '../utils.ts';

interface MermaidViewerPayload {
  source: string;
  canEdit: boolean;
  fileName: string;
}

let registered = false;

function parsePayload(script: HTMLScriptElement | null): MermaidViewerPayload | null {
  if (!script?.textContent) return null;
  try {
    const raw = JSON.parse(script.textContent) as Partial<MermaidViewerPayload>;
    if (!raw || typeof raw.source !== 'string') return null;
    return {
      source: raw.source,
      canEdit: raw.canEdit ?? false,
      fileName: raw.fileName ?? 'diagram.mmd',
    };
  } catch {
    return null;
  }
}

export function initRepoMermaidViewer(): void {
  if (registered) return;
  registered = true;

  registerGlobalInitFunc('initRepoMermaidViewer', async (container: HTMLElement) => {
    const script = container.querySelector<HTMLScriptElement>('#mermaid-viewer-payload');
    const payload = parsePayload(script);
    if (!payload) return;

    const {default: mermaid} = await import(/* webpackChunkName: "mermaid" */'mermaid');

    mermaid.initialize({
      startOnLoad: false,
      theme: isDarkTheme() ? 'dark' : 'neutral',
      securityLevel: 'strict',
      suppressErrorRendering: true,
    });

    let currentZoom = 1;
    const originalSource = payload.source;
    let renderCounter = 0;

    // --- Rendering ---
    async function renderDiagram(containerId: string, source: string): Promise<void> {
      const el = container.querySelector<HTMLElement>(`#${containerId}`);
      const errorDiv = container.querySelector<HTMLElement>('#mermaid-error');
      const errorMsg = container.querySelector<HTMLElement>('#mermaid-error-message');
      if (!el) return;

      el.innerHTML = '<div class="ui active centered inline loader"></div>';
      errorDiv?.classList.add('tw-hidden');

      try {
        await mermaid.parse(source);
        renderCounter++;
        const diagramId = `mermaid-diag-${containerId}-${renderCounter}`;
        const {svg} = await mermaid.render(diagramId, source);
        el.innerHTML = svg;

        const svgElement = el.querySelector('svg');
        if (svgElement && containerId === 'mermaid-preview-container') {
          applyZoomToSvg(svgElement);
        }
      } catch (err: any) {
        el.innerHTML = '';
        errorDiv?.classList.remove('tw-hidden');
        if (errorMsg) {
          errorMsg.textContent = err.message || 'Unknown rendering error';
        }
      }
    }

    // --- Source view ---
    function buildSourceView(): void {
      const sourceView = container.querySelector<HTMLElement>('#mermaid-source-view');
      if (!sourceView) return;

      const lines = originalSource.split('\n');
      const table = document.createElement('table');
      table.className = 'chroma';
      const tbody = document.createElement('tbody');

      for (let i = 0; i < lines.length; i++) {
        const tr = document.createElement('tr');

        const tdNum = document.createElement('td');
        tdNum.className = 'lines-num';
        const span = document.createElement('span');
        span.setAttribute('data-line-number', String(i + 1));
        tdNum.appendChild(span);

        const tdCode = document.createElement('td');
        tdCode.className = 'lines-code chroma';
        const code = document.createElement('code');
        code.className = 'code-inner';
        code.textContent = lines[i];
        tdCode.appendChild(code);

        tr.appendChild(tdNum);
        tr.appendChild(tdCode);
        tbody.appendChild(tr);
      }

      table.appendChild(tbody);
      sourceView.innerHTML = '';
      sourceView.appendChild(table);
    }

    // --- Tabs ---
    function switchPanel(panelName: string): void {
      const panels = container.querySelectorAll<HTMLElement>('.mermaid-panel');
      for (const p of panels) {
        p.classList.add('tw-hidden');
      }
      const target = container.querySelector<HTMLElement>(`#mermaid-${panelName}-panel`);
      target?.classList.remove('tw-hidden');

      const buttons = container.querySelectorAll<HTMLElement>('.mermaid-mode-buttons .button');
      for (const btn of buttons) {
        btn.classList.toggle('active', btn.getAttribute('data-mermaid-action') === panelName);
      }
    }

    // --- Zoom ---
    function applyZoomToSvg(svg: SVGElement): void {
      svg.style.transform = `scale(${currentZoom})`;
      svg.style.transformOrigin = 'top left';
      svg.style.transition = 'transform 0.2s ease';
    }

    function applyZoom(): void {
      const svg = container.querySelector<SVGElement>('#mermaid-preview-container svg');
      if (svg) applyZoomToSvg(svg);
    }

    // --- Export ---
    function downloadBlob(blob: Blob, fileName: string): void {
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    }

    function exportDiagram(format: 'svg' | 'png'): void {
      const svg = container.querySelector<SVGElement>('#mermaid-preview-container svg');
      if (!svg) return;

      const baseName = payload.fileName.replace(/\.mmd$/i, '');

      if (format === 'svg') {
        const svgData = new XMLSerializer().serializeToString(svg);
        const blob = new Blob([svgData], {type: 'image/svg+xml'});
        downloadBlob(blob, `${baseName}.svg`);
      } else {
        const svgData = new XMLSerializer().serializeToString(svg);
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        const img = new Image();
        img.onload = () => {
          canvas.width = img.width * 2;
          canvas.height = img.height * 2;
          ctx?.scale(2, 2);
          ctx?.drawImage(img, 0, 0);
          canvas.toBlob((blob) => {
            if (blob) downloadBlob(blob, `${baseName}.png`);
          });
        };
        img.src = `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(svgData)))}`;
      }
    }

    // --- Wire up toolbar buttons ---
    container.addEventListener('click', (e: Event) => {
      const target = e.target as HTMLElement;
      const btn = target.closest<HTMLElement>('[data-mermaid-action]');
      if (!btn) return;

      const action = btn.getAttribute('data-mermaid-action');
      switch (action) {
        case 'preview':
        case 'source':
        case 'edit':
          switchPanel(action);
          break;
        case 'zoom-in':
          currentZoom = Math.min(currentZoom + 0.1, 3);
          applyZoom();
          break;
        case 'zoom-out':
          currentZoom = Math.max(currentZoom - 0.1, 0.3);
          applyZoom();
          break;
        case 'zoom-reset':
          currentZoom = 1;
          applyZoom();
          break;
        case 'export-svg':
          exportDiagram('svg');
          break;
        case 'export-png':
          exportDiagram('png');
          break;
        case 'apply': {
          const editor = container.querySelector<HTMLTextAreaElement>('#mermaid-editor');
          if (editor) {
            renderDiagram('mermaid-preview-container', editor.value);
            switchPanel('preview');
          }
          break;
        }
        case 'reset': {
          const editor = container.querySelector<HTMLTextAreaElement>('#mermaid-editor');
          if (editor) {
            editor.value = originalSource;
            renderDiagram('mermaid-live-preview', originalSource);
          }
          break;
        }
      }
    });

    // Mouse wheel zoom on preview
    const previewContainer = container.querySelector<HTMLElement>('#mermaid-preview-container');
    previewContainer?.addEventListener('wheel', (e: WheelEvent) => {
      e.preventDefault();
      if (e.deltaY < 0) {
        currentZoom = Math.min(currentZoom + 0.05, 3);
      } else {
        currentZoom = Math.max(currentZoom - 0.05, 0.3);
      }
      applyZoom();
    });

    // --- Editor with live preview ---
    if (payload.canEdit) {
      const editor = container.querySelector<HTMLTextAreaElement>('#mermaid-editor');
      if (editor) {
        editor.value = originalSource;

        let debounceTimer: ReturnType<typeof setTimeout>;
        editor.addEventListener('input', () => {
          clearTimeout(debounceTimer);
          debounceTimer = setTimeout(() => {
            renderDiagram('mermaid-live-preview', editor.value);
          }, 800);
        });
      }
    }

    // --- Initial renders ---
    buildSourceView();
    await renderDiagram('mermaid-preview-container', originalSource);
    if (payload.canEdit) {
      renderDiagram('mermaid-live-preview', originalSource);
    }
  });
}
