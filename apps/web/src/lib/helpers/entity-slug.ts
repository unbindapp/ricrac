/** The URL slug the server gives projects, initiatives and documents: `launch-v2-0a1b2c3d4e5f`. */
export function entitySlug(entity: { slugId: string; name?: string; title?: string }) {
  const words = (entity.name ?? entity.title ?? "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 60)
    .replace(/^-+|-+$/g, "");
  return words ? `${words}-${entity.slugId}` : entity.slugId;
}

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** The slug id ending a link slug, so a renamed thing keeps one cache entry; ids pass as they are. */
export function slugIdFromParam(param: string) {
  if (uuidPattern.test(param)) return param;
  return param.split("-").at(-1) ?? param;
}
