import type {ComplexType, ElementDecl, Occurs, SchemaDoc} from './types.ts';

function updateElementRefs(doc: SchemaDoc, oldName: string, newName: string) {
  const updateElement = (element: ElementDecl) => {
    if (element.name === oldName) element.name = newName;
    if (element.type === oldName) element.type = newName;
    if (element.children) {
      element.children.forEach((particle) => {
        if (particle.element) updateElement(particle.element);
        if (particle.ref === oldName) particle.ref = newName;
      });
    }
  };

  doc.elements.forEach(updateElement);
  doc.types.forEach((type) => {
    type.sequence?.forEach((particle) => {
      if (particle.element) updateElement(particle.element);
      if (particle.ref === oldName) particle.ref = newName;
    });
    type.choice?.forEach((particle) => {
      if (particle.element) updateElement(particle.element);
      if (particle.ref === oldName) particle.ref = newName;
    });
  });
}

function updateTypeRefs(doc: SchemaDoc, oldName: string, newName: string) {
  doc.types.forEach((type) => {
    if (type.name === oldName) type.name = newName;
    if (type.base === oldName) type.base = newName;
  });
  const updateElementType = (element: ElementDecl) => {
    if (element.type === oldName) element.type = newName;
    if (element.children) {
      element.children.forEach((particle) => {
        if (particle.element) updateElementType(particle.element);
      });
    }
  };
  doc.elements.forEach(updateElementType);
  doc.types.forEach((type) => {
    type.sequence?.forEach((particle) => {
      if (particle.element) updateElementType(particle.element);
    });
    type.choice?.forEach((particle) => {
      if (particle.element) updateElementType(particle.element);
    });
  });
}

export function renameElement(doc: SchemaDoc, oldName: string, newName: string) {
  updateElementRefs(doc, oldName, newName);
}

export function renameType(doc: SchemaDoc, oldName: string, newName: string) {
  updateTypeRefs(doc, oldName, newName);
}

export function setOccurs(element: ElementDecl, minOccurs?: number, maxOccurs?: Occurs) {
  element.minOccurs = minOccurs;
  element.maxOccurs = maxOccurs;
}

export function setDocumentation(target: ElementDecl | ComplexType, text: string) {
  target.annotation = text;
}

export function addChildElement(
  doc: SchemaDoc,
  parentTypeName: string,
  childName: string,
  childTypeQName: string,
  minOccurs?: number,
  maxOccurs?: Occurs,
): boolean {
  const parent = doc.types.find((type) => type.name === parentTypeName);
  if (!parent || !parent.sequence) return false;
  parent.sequence.push({
    kind: 'elementInline',
    element: {
      name: childName,
      type: childTypeQName,
      minOccurs,
      maxOccurs,
    },
  });
  return true;
}
