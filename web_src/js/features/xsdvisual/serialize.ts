import type {ComplexType, ElementDecl, Occurs, Particle, SchemaDoc} from './types.ts';

const SCHEMA_NS = 'http://www.w3.org/2001/XMLSchema';

function setOccurs(el: Element, minOccurs?: number, maxOccurs?: Occurs) {
  if (minOccurs !== undefined) el.setAttribute('minOccurs', String(minOccurs));
  if (maxOccurs !== undefined) el.setAttribute('maxOccurs', String(maxOccurs));
}

function appendAnnotation(doc: Document, target: Element, text?: string) {
  if (!text) return;
  const annotation = doc.createElementNS(SCHEMA_NS, 'xs:annotation');
  const documentation = doc.createElementNS(SCHEMA_NS, 'xs:documentation');
  documentation.textContent = text;
  annotation.append(documentation);
  target.append(annotation);
}

function appendParticle(doc: Document, parent: Element, particle: Particle) {
  if (particle.kind === 'elementInline' && particle.element) {
    const element = doc.createElementNS(SCHEMA_NS, 'xs:element');
    element.setAttribute('name', particle.element.name);
    if (particle.element.type) element.setAttribute('type', particle.element.type);
    setOccurs(element, particle.element.minOccurs, particle.element.maxOccurs);
    appendAnnotation(doc, element, particle.element.annotation);
    parent.append(element);
    return;
  }
  if (particle.kind === 'elementRef' && particle.ref) {
    const element = doc.createElementNS(SCHEMA_NS, 'xs:element');
    element.setAttribute('ref', particle.ref);
    setOccurs(element, particle.minOccurs, particle.maxOccurs);
    parent.append(element);
    return;
  }
  if (particle.kind === 'group') {
    const group = doc.createElementNS(SCHEMA_NS, 'xs:group');
    if (particle.ref) group.setAttribute('ref', particle.ref);
    setOccurs(group, particle.minOccurs, particle.maxOccurs);
    parent.append(group);
  }
  if (particle.kind === 'any') {
    const any = doc.createElementNS(SCHEMA_NS, 'xs:any');
    setOccurs(any, particle.minOccurs, particle.maxOccurs);
    parent.append(any);
  }
}

function appendSequence(doc: Document, parent: Element, particles?: Particle[]) {
  if (!particles || particles.length === 0) return;
  const sequence = doc.createElementNS(SCHEMA_NS, 'xs:sequence');
  particles.forEach((particle) => appendParticle(doc, sequence, particle));
  parent.append(sequence);
}

function appendChoice(doc: Document, parent: Element, particles?: Particle[]) {
  if (!particles || particles.length === 0) return;
  const choice = doc.createElementNS(SCHEMA_NS, 'xs:choice');
  particles.forEach((particle) => appendParticle(doc, choice, particle));
  parent.append(choice);
}

function appendComplexType(doc: Document, parent: Element, type: ComplexType) {
  const complexType = doc.createElementNS(SCHEMA_NS, 'xs:complexType');
  complexType.setAttribute('name', type.name);
  appendAnnotation(doc, complexType, type.annotation);
  if (type.base) {
    const complexContent = doc.createElementNS(SCHEMA_NS, 'xs:complexContent');
    const extension = doc.createElementNS(SCHEMA_NS, 'xs:extension');
    extension.setAttribute('base', type.base);
    appendSequence(doc, extension, type.sequence);
    appendChoice(doc, extension, type.choice);
    complexContent.append(extension);
    complexType.append(complexContent);
  } else {
    appendSequence(doc, complexType, type.sequence);
    appendChoice(doc, complexType, type.choice);
  }
  parent.append(complexType);
}

function appendElement(doc: Document, parent: Element, elementDecl: ElementDecl) {
  const element = doc.createElementNS(SCHEMA_NS, 'xs:element');
  element.setAttribute('name', elementDecl.name);
  if (elementDecl.type) element.setAttribute('type', elementDecl.type);
  setOccurs(element, elementDecl.minOccurs, elementDecl.maxOccurs);
  appendAnnotation(doc, element, elementDecl.annotation);
  if (elementDecl.children && elementDecl.children.length > 0) {
    const inlineType = doc.createElementNS(SCHEMA_NS, 'xs:complexType');
    appendSequence(doc, inlineType, elementDecl.children);
    element.append(inlineType);
  }
  parent.append(element);
}

function appendSimpleType(doc: Document, parent: Element, name: string, annotation?: string) {
  const simpleType = doc.createElementNS(SCHEMA_NS, 'xs:simpleType');
  simpleType.setAttribute('name', name);
  appendAnnotation(doc, simpleType, annotation);
  parent.append(simpleType);
}

function formatXmlNode(node: Node, depth = 0): string[] {
  const indentUnit = '  ';
  const indent = indentUnit.repeat(depth);
  const lines: string[] = [];

  if (node.nodeType === Node.DOCUMENT_NODE) {
    const doc = node as Document;
    doc.childNodes.forEach((child) => lines.push(...formatXmlNode(child, depth)));
    return lines;
  }

  if (node.nodeType !== Node.ELEMENT_NODE) return lines;
  const element = node as Element;
  const attrs = Array.from(element.attributes)
    .map((attr) => ` ${attr.name}="${attr.value}"`)
    .join('');
  const children = Array.from(element.childNodes).filter((child) => child.nodeType !== Node.TEXT_NODE || child.textContent?.trim());

  if (children.length === 0) {
    lines.push(`${indent}<${element.tagName}${attrs}/>`);
    return lines;
  }

  lines.push(`${indent}<${element.tagName}${attrs}>`);
  children.forEach((child) => {
    if (child.nodeType === Node.TEXT_NODE) {
      const text = child.textContent?.trim();
      if (text) lines.push(`${indent}${indentUnit}${text}`);
      return;
    }
    lines.push(...formatXmlNode(child, depth + 1));
  });
  lines.push(`${indent}</${element.tagName}>`);
  return lines;
}

export function serializeXsd(doc: SchemaDoc): string {
  const xmlDoc = document.implementation.createDocument('', '', null);
  const schema = xmlDoc.createElementNS(SCHEMA_NS, 'xs:schema');
  schema.setAttribute('xmlns:xs', SCHEMA_NS);
  if (doc.targetNamespace) schema.setAttribute('targetNamespace', doc.targetNamespace);
  xmlDoc.append(schema);

  doc.simpleTypes.forEach((simpleType) => appendSimpleType(xmlDoc, schema, simpleType.name, simpleType.annotation));
  doc.types.forEach((type) => appendComplexType(xmlDoc, schema, type));
  doc.elements.forEach((element) => appendElement(xmlDoc, schema, element));

  return `${formatXmlNode(xmlDoc).join('\n')}\n`;
}
