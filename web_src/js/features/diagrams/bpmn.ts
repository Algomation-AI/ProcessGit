// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnViewer from 'bpmn-js/dist/bpmn-viewer.production.min.js';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import BpmnModeler from 'bpmn-js/lib/Modeler';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - upstream packages do not ship full type definitions
import {BpmnPropertiesPanelModule, BpmnPropertiesProviderModule} from 'bpmn-js-properties-panel';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore - package ships without types
import * as AutoLayoutPkg from 'bpmn-auto-layout';
import type {DiagramAdapter} from './types.ts';

import 'bpmn-js/dist/assets/diagram-js.css';
import 'bpmn-js/dist/assets/bpmn-js.css';
import 'bpmn-js/dist/assets/bpmn-font/css/bpmn.css';
import '@bpmn-io/properties-panel/dist/assets/properties-panel.css';

console.debug('[bpmn] bpmn-auto-layout exports:', AutoLayoutPkg);

function createAutoLayout(): any {
  const anyPkg: any = AutoLayoutPkg;
  const Ctor = anyPkg?.default ?? anyPkg?.AutoLayout ?? anyPkg;

  if (typeof Ctor !== 'function') {
    throw new Error('bpmn-auto-layout export is not a constructor');
  }

  return new Ctor();
}

function clearContainer(container: HTMLElement, properties?: HTMLElement | null) {
  container.innerHTML = '';
  properties?.classList.add('tw-hidden');
  if (properties) properties.innerHTML = '';
}

async function prepareBpmnXml(xml: string): Promise<string> {
  let xmlTrim = (xml || '').trim();

  if (!xmlTrim) {
    throw new Error('no diagram to display: empty xml');
  }

  if (xmlTrim.startsWith('<!DOCTYPE html') || xmlTrim.startsWith('<html')) {
    throw new Error('no diagram to display: got HTML instead of BPMN XML');
  }

  if (!xmlTrim.includes('<bpmn:definitions') && !xmlTrim.includes('xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL"')) {
    throw new Error('no diagram to display: content is not BPMN XML');
  }

  if (!xmlTrim.includes('<bpmn:process') && !xmlTrim.includes('<bpmn:collaboration')) {
    throw new Error('no diagram to display: BPMN definitions without process/collaboration');
  }

  if (!xmlTrim.includes('bpmndi:BPMNDiagram')) {
    try {
      const autoLayout = createAutoLayout();
      xmlTrim = await autoLayout.layoutProcess(xmlTrim);
    } catch (e) {
      console.warn('[bpmn] auto-layout failed, importing original XML', e);
    }
  }

  return xmlTrim;
}

export function createBpmnAdapter(canvas: HTMLElement, properties?: HTMLElement | null): DiagramAdapter {
  let viewer: any = null;
  let modeler: any = null;
  let changeHandler: (() => void) | null = null;
  let removeChangeHandler: (() => void) | null = null;
  const getPropertiesContainer = () => properties ?? document.querySelector<HTMLElement>('#diagram-properties');

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
      clearContainer(canvas, getPropertiesContainer());
      viewer = new BpmnViewer({container: canvas});
      const preparedXml = await prepareBpmnXml(xml);
      await viewer.importXML(preparedXml);
      viewer.get('canvas')?.zoom('fit-viewport');
    },

    async enterEdit(xml: string) {
      cleanupViewer();
      cleanupModeler();
      const propertiesPanelParent = getPropertiesContainer();
      clearContainer(canvas, propertiesPanelParent);
      modeler = new BpmnModeler({
        container: canvas,
        propertiesPanel: propertiesPanelParent ? {parent: '#diagram-properties'} : undefined,
        additionalModules: propertiesPanelParent ? [BpmnPropertiesPanelModule, BpmnPropertiesProviderModule] : undefined,
      });
      const preparedXml = await prepareBpmnXml(xml);
      await modeler.importXML(preparedXml);
      modeler.get('canvas')?.zoom('fit-viewport');
      propertiesPanelParent?.classList.remove('tw-hidden');
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
