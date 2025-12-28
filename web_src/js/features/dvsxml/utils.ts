export function normalizeKey(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '');
}

export function trimToUndefined(val?: string | null): string | undefined {
  if (!val) return undefined;
  const trimmed = val.trim();
  return trimmed ? trimmed : undefined;
}

export function getChildElements(element: Element): Element[] {
  return Array.from(element.children || []);
}

export function findChild(element: Element, names: string[]): Element | null {
  const normalizedNames = new Set(names.map(normalizeKey));
  return getChildElements(element).find((child) => normalizedNames.has(normalizeKey(child.localName))) ?? null;
}

export function findChildren(element: Element, names: string[]): Element[] {
  const normalizedNames = new Set(names.map(normalizeKey));
  return getChildElements(element).filter((child) => normalizedNames.has(normalizeKey(child.localName)));
}

export function childText(element: Element | null, names: string[]): string | undefined {
  if (!element) return undefined;
  const child = findChild(element, names);
  if (!child) return undefined;
  return trimToUndefined(child.textContent);
}

export function firstAttribute(element: Element, names: string[], used: Set<string>): string | undefined {
  const normalizedNames = new Set(names.map(normalizeKey));
  for (const attr of Array.from(element.attributes)) {
    const normalized = normalizeKey(attr.name);
    if (normalizedNames.has(normalized)) {
      used.add(normalized);
      return trimToUndefined(attr.value);
    }
  }
  return undefined;
}

export function collectUnknownAttributes(element: Element, used: Set<string>): Record<string, string> {
  const unknown: Record<string, string> = {};
  for (const attr of Array.from(element.attributes)) {
    const normalized = normalizeKey(attr.name);
    if (normalized.startsWith('xmlns')) continue;
    if (used.has(normalized)) continue;
    unknown[attr.name] = attr.value;
  }
  return unknown;
}

export function collectUnknownChildren(element: Element, allowed: Set<string>): string[] {
  const unknown: string[] = [];
  for (const child of getChildElements(element)) {
    const normalized = normalizeKey(child.localName);
    if (allowed.has(normalized)) continue;
    unknown.push(child.outerHTML || child.textContent || child.localName);
  }
  return unknown;
}

export function fallback(value?: string, alt?: string): string | undefined {
  return trimToUndefined(value) ?? trimToUndefined(alt);
}
