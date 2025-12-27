// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import DmnViewer from 'dmn-js/lib/Viewer';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import DmnModeler from 'dmn-js/lib/Modeler';
import type {DiagramAdapter} from './types.ts';

import 'dmn-js/dist/assets/diagram-js.css';
import 'dmn-js/dist/assets/dmn-font/css/dmn.css';
import 'dmn-js/dist/assets/dmn-js-decision-table.css';
import 'dmn-js/dist/assets/dmn-js-drd.css';
import 'dmn-js/dist/assets/dmn-js-literal-expression.css';
import 'dmn-js/dist/assets/dmn-js-shared.css';
import 'dmn-js/dist/assets/dmn-js-decision-table-controls.css';

function clearContainer(container: HTMLElement, properties?: HTMLElement | null) {
  container.innerHTML = '';
  properties?.classList.add('tw-hidden');
  if (properties) properties.innerHTML = '';
}

function fitDmnViewport(viewer: any) {
  const activeViewer = viewer?.getActiveViewer?.();
  const canvas = activeViewer?.get?.('canvas');
  canvas?.zoom?.('fit-viewport');
}

export function createDmnAdapter(canvas: HTMLElement, properties?: HTMLElement | null): DiagramAdapter {
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
      viewer = new DmnViewer({container: canvas});
      await viewer.importXML(xml);
      fitDmnViewport(viewer);
    },

    async enterEdit(xml: string) {
      cleanupViewer();
      cleanupModeler();
      clearContainer(canvas, properties);
      modeler = new DmnModeler({container: canvas});
      await modeler.importXML(xml);
      fitDmnViewport(modeler);
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
