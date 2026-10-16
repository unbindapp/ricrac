/** A URL key from a workspace name: "Acme Inc." → "acme-inc". */
export const urlKeyFrom = (name: string) =>
  name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 32)
    .replace(/-+$/, "")

/** A team key from a team name, as the server derives it: "Engineering" → "ENG", "Mobile Apps" → "MA". */
export function teamKeyFrom(name: string) {
  const words = name.split(/\s+/).filter((w) => /[\p{L}\p{N}]/u.test(w))
  const key =
    words.length > 1
      ? words
          .map((w) => w.match(/[A-Za-z0-9]/)?.[0] ?? "")
          .join("")
          .slice(0, 5)
      : name.replace(/[^A-Za-z0-9]/g, "").slice(0, 3)
  return key.toUpperCase()
}

/** The link slug of a project, initiative or document: its name, then its slug id. */
export const slugFor = (ref: { name: string; slugId: string }) =>
  [urlKeyFrom(ref.name), ref.slugId].filter(Boolean).join("-")
