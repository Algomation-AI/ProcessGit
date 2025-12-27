// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnViewer from 'bpmn-js/dist/bpmn-viewer.production.min.js';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnModeler from 'bpmn-js/lib/Modeler';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnPropertiesPanelModule from 'bpmn-js-properties-panel';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnPropertiesProviderModule from 'bpmn-js-properties-panel/lib/provider/bpmn';
import type {DiagramAdapter} from './types.ts';

import 'bpmn-js/dist/assets/diagram-js.css';
import 'bpmn-js/dist/assets/bpmn-js.css';
import 'bpmn-js/dist/assets/bpmn-font/css/bpmn-embedded.css';
import 'bpmn-js-properties-panel/dist/assets/properties-panel.css';

function clearContainer(container: HTMLElement, properties?: HTMLElement | null) {
  container.innerHTML = '';
  properties?.classList.add('tw-hidden');
  if (properties) properties.innerHTML = '';
}

export function createBpmnAdapter(canvas: HTMLElement, properties?: HTMLElement | null): DiagramAdapter {
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
      viewer = new BpmnViewer({container: canvas});
      await viewer.importXML(xml);
      viewer.get('canvas')?.zoom('fit-viewport');
    },

    async enterEdit(xml: string) {
      cleanupViewer();
      cleanupModeler();
      clearContainer(canvas, properties);
      modeler = new BpmnModeler({
        container: canvas,
        propertiesPanel: properties ? {parent: properties} : undefined,
        additionalModules: properties ? [BpmnPropertiesPanelModule, BpmnPropertiesProviderModule] : undefined,
      });
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
