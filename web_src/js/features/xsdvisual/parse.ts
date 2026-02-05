import type {AttributeDecl, ComplexType, ElementDecl, ParsedXsd, Particle, SchemaDoc} from './types.ts';

const SCHEMA_NS = 'http://www.w3.org/2001/XMLSchema';

function isSchemaNode(node: Element | null, localName: string): node is Element {
  if (!node) return false;
  if (node.localName !== localName) return false;
  return !node.namespaceURI || node.namespaceURI === SCHEMA_NS;
}

function parseOccurs(value: string | null): number | 'unbounded' | undefined {
  if (!value) return undefined;
  if (value === 'unbounded') return 'unbounded';
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function parseAnnotationText(element: Element): string | undefined {
  for (const child of Array.from(element.children)) {
    if (isSchemaNode(child, 'annotation')) {
      for (const doc of Array.from(child.children)) {
        if (isSchemaNode(doc, 'documentation')) {
          return doc.textContent?.trim() || undefined;
        }
      }
    }
  }
  return undefined;
}

function getAttr(element: Element, name: string): string | undefined {
  const value = element.getAttribute(name);
  return value ? value : undefined;
}

function parseAttributes(container: Element): AttributeDecl[] {
  const attrs: AttributeDecl[] = [];
  for (const child of Array.from(container.children)) {
    if (!isSchemaNode(child, 'attribute')) continue;
    attrs.push({
      name: getAttr(child, 'name'),
      ref: getAttr(child, 'ref'),
      type: getAttr(child, 'type'),
      use: getAttr(child, 'use'),
    });
  }
  return attrs;
}

function parseElementDecl(element: Element, warnings: string[]): ElementDecl {
  const name = element.getAttribute('name') ?? element.getAttribute('ref') ?? 'unnamed';
  const type = element.getAttribute('type') ?? undefined;
  const minOccurs = parseOccurs(element.getAttribute('minOccurs')) as number | undefined;
  const maxOccurs = parseOccurs(element.getAttribute('maxOccurs'));
  const annotation = parseAnnotationText(element);

  let children: Particle[] | undefined;
  let attributes: AttributeDecl[] | undefined;
  for (const child of Array.from(element.children)) {
    if (isSchemaNode(child, 'complexType')) {
      const inlineType = parseComplexType(child, warnings, true);
      children = inlineType.sequence ?? inlineType.choice;
      attributes = inlineType.attributes;
      break;
    }
  }

  return {
    name,
    type,
    minOccurs,
    maxOccurs,
    annotation,
    children,
    attributes,
  };
}

function parseParticles(container: Element, warnings: string[]): Particle[] {
  const particles: Particle[] = [];

  for (const child of Array.from(container.children)) {
    if (isSchemaNode(child, 'element')) {
      const ref = child.getAttribute('ref') ?? undefined;
      const elementDecl = ref ? undefined : parseElementDecl(child, warnings);
      const minOccurs = parseOccurs(child.getAttribute('minOccurs')) as number | undefined;
      const maxOccurs = parseOccurs(child.getAttribute('maxOccurs'));
      particles.push({
        kind: ref ? 'elementRef' : 'elementInline',
        ref,
        element: elementDecl,
        minOccurs,
        maxOccurs,
      });
      continue;
    }
    if (isSchemaNode(child, 'group')) {
      particles.push({
        kind: 'group',
        ref: child.getAttribute('ref') ?? undefined,
        minOccurs: parseOccurs(child.getAttribute('minOccurs')) as number | undefined,
        maxOccurs: parseOccurs(child.getAttribute('maxOccurs')),
      });
      continue;
    }
    if (isSchemaNode(child, 'any')) {
      particles.push({
        kind: 'any',
        minOccurs: parseOccurs(child.getAttribute('minOccurs')) as number | undefined,
        maxOccurs: parseOccurs(child.getAttribute('maxOccurs')),
      });
      continue;
    }
    warnings.push(`Ignored unsupported particle: <${child.tagName}>`);
  }

  return particles;
}

function parseComplexType(element: Element, warnings: string[], inline = false): ComplexType {
  const name = element.getAttribute('name') ?? (inline ? 'inline' : 'unnamed');
  const annotation = parseAnnotationText(element);
  const attributes = parseAttributes(element);

  let base: string | undefined;
  let sequence: Particle[] | undefined;
  let choice: Particle[] | undefined;

  for (const child of Array.from(element.children)) {
    if (isSchemaNode(child, 'complexContent')) {
      for (const ccChild of Array.from(child.children)) {
        if (isSchemaNode(ccChild, 'extension')) {
          base = ccChild.getAttribute('base') ?? undefined;
          for (const extChild of Array.from(ccChild.children)) {
            if (isSchemaNode(extChild, 'sequence')) {
              sequence = parseParticles(extChild, warnings);
            } else if (isSchemaNode(extChild, 'choice')) {
              choice = parseParticles(extChild, warnings);
            }
          }
        }
      }
      continue;
    }

    if (isSchemaNode(child, 'sequence')) {
      sequence = parseParticles(child, warnings);
      continue;
    }
    if (isSchemaNode(child, 'choice')) {
      choice = parseParticles(child, warnings);
      continue;
    }
  }

  return {
    name,
    base,
    sequence,
    choice,
    annotation,
    attributes,
  };
}

export function parseXsd(xmlText: string): ParsedXsd {
  const warnings: string[] = [];
  const parser = new DOMParser();
  const doc = parser.parseFromString(xmlText, 'application/xml');
  if (doc.getElementsByTagName('parsererror').length) {
    throw new Error('XML parser error');
  }

  const schema = doc.documentElement;
  if (!isSchemaNode(schema, 'schema')) {
    throw new Error('Root element is not xs:schema');
  }

  const schemaDoc: SchemaDoc = {
    targetNamespace: schema.getAttribute('targetNamespace') ?? undefined,
    elements: [],
    types: [],
    simpleTypes: [],
    annotations: [],
    includes: [],
    imports: [],
  };

  for (const child of Array.from(schema.children)) {
    if (isSchemaNode(child, 'element')) {
      schemaDoc.elements.push(parseElementDecl(child, warnings));
      continue;
    }
    if (isSchemaNode(child, 'complexType')) {
      schemaDoc.types.push(parseComplexType(child, warnings));
      continue;
    }
    if (isSchemaNode(child, 'simpleType')) {
      const name = child.getAttribute('name') ?? 'unnamed';
      schemaDoc.simpleTypes.push({name, annotation: parseAnnotationText(child)});
      continue;
    }
    if (isSchemaNode(child, 'annotation')) {
      schemaDoc.annotations.push({documentation: parseAnnotationText(child)});
      continue;
    }
    if (isSchemaNode(child, 'include')) {
      const schemaLocation = child.getAttribute('schemaLocation');
      if (schemaLocation) schemaDoc.includes.push(schemaLocation);
      continue;
    }
    if (isSchemaNode(child, 'import')) {
      schemaDoc.imports.push({
        namespace: child.getAttribute('namespace') ?? undefined,
        schemaLocation: child.getAttribute('schemaLocation') ?? undefined,
      });
      continue;
    }
    warnings.push(`Ignored unsupported schema child: <${child.tagName}>`);
  }

  return {doc: schemaDoc, warnings};
}
