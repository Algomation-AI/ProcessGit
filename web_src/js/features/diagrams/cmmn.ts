// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import CmmnViewer from 'cmmn-js/lib/Viewer';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import CmmnModeler from 'cmmn-js/lib/Modeler';
import type {DiagramAdapter} from './types.ts';

import 'cmmn-js/dist/assets/diagram-js.css';
import 'cmmn-js/dist/assets/cmmn-font/css/cmmn.css';

function clearContainer(container: HTMLElement, properties?: HTMLElement | null) {
  container.innerHTML = '';
  properties?.classList.add('tw-hidden');
  if (properties) properties.innerHTML = '';
}

export function createCmmnAdapter(canvas: HTMLElement, properties?: HTMLElement | null): DiagramAdapter {
  let viewer: any = null;
  let modeler: any = null;
  let changeHandler: (() => void) | null = null;
  let removeChangeHandler: (() => void) | null = null;

  const cleanupViewer = () => {
    if (viewer?.destroy) viewer.destroy();
    viewer = null;
  };

  const cleanupModeler = () => {
    removeChangeHandler?.();
    removeChangeHandler = null;
    if (modeler?.destroy) modeler.destroy();
    modeler = null;
  };

  const bindChangeHandler = () => {
    if (!modeler || !changeHandler) return;
    const eventBus = modeler.get('eventBus');
    const handler = () => changeHandler?.();
    removeChangeHandler?.();
    eventBus.on('commandStack.changed', handler);
    removeChangeHandler = () => eventBus.off('commandStack.changed', handler);
  };

  return {
    async renderPreview(xml: string) {
      cleanupModeler();
      cleanupViewer();
      clearContainer(canvas, properties);
      viewer = new CmmnViewer({container: canvas});
      await viewer.importXML(xml);
      viewer.get('canvas')?.zoom('fit-viewport');
    },

    async enterEdit(xml: string) {
      cleanupViewer();
      cleanupModeler();
      clearContainer(canvas, properties);
      modeler = new CmmnModeler({container: canvas});
      await modeler.importXML(xml);
      modeler.get('canvas')?.zoom('fit-viewport');
      if (properties) properties.classList.remove('tw-hidden');
      bindChangeHandler();
    },

    async save() {
      if (!modeler) throw new Error('Diagram editor is not ready');
      const {xml} = await modeler.saveXML({format: true});
      return xml;
    },

    setChangeHandler(handler: () => void) {
      changeHandler = handler;
      bindChangeHandler();
    },

    destroy() {
      cleanupViewer();
      cleanupModeler();
    },
  };
}
