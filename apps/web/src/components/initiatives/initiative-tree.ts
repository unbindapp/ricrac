type TNode = { id: string; parent: { id: string } | null };

export type TNested<T> = { item: T; depth: number };

/** Orders initiatives so each follows its parent, indented one level deeper per ancestor shown. */
export function nestInitiatives<T extends TNode>(items: T[]): TNested<T>[] {
  const ids = new Set(items.map((item) => item.id));
  const children = new Map<string, T[]>();
  for (const item of items) {
    if (!item.parent || !ids.has(item.parent.id)) continue;
    children.set(item.parent.id, [...(children.get(item.parent.id) ?? []), item]);
  }
  const result: TNested<T>[] = [];
  const seen = new Set<string>();
  const visit = (item: T, depth: number) => {
    if (seen.has(item.id)) return;
    seen.add(item.id);
    result.push({ item, depth });
    for (const child of children.get(item.id) ?? []) visit(child, depth + 1);
  };
  for (const item of items) {
    if (!item.parent || !ids.has(item.parent.id)) visit(item, 0);
  }
  for (const item of items) visit(item, 0);
  return result;
}

/** The ids of an initiative and everything below it, which can't become its parent. */
export function descendantIds<T extends TNode>(items: T[], rootId: string) {
  const result = new Set([rootId]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const item of items) {
      if (item.parent && result.has(item.parent.id) && !result.has(item.id)) {
        result.add(item.id);
        grew = true;
      }
    }
  }
  return result;
}
