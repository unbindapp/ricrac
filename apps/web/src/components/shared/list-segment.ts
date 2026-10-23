/** Which ends of a segmented list a row sits at, for its outer corners in Dank. */
export function segmentOf(index: number, count: number) {
  const edges = [index === 0 && "first", index === count - 1 && "last"].filter(Boolean);
  return edges.length > 0 ? edges.join(" ") : undefined;
}
