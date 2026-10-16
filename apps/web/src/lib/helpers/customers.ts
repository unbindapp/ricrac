const domainPattern = /^(?=.{1,253}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$/;

export function isDomain(value: string) {
  return domainPattern.test(value);
}

/** "acme.com, https://www.acme.io/pricing" → ["acme.com", "acme.io"]. */
export function parseDomains(text: string) {
  return text
    .split(/[\s,]+/)
    .map((domain) =>
      domain
        .trim()
        .toLowerCase()
        .replace(/^https?:\/\//, "")
        .replace(/^www\./, "")
        .replace(/\/.*$/, ""),
    )
    .filter((domain, index, all) => domain !== "" && all.indexOf(domain) === index);
}

/** "1,200" → 1200, "" → null, anything else → undefined. */
export function parseWholeNumber(text: string): number | null | undefined {
  const digits = text.replace(/[,_\s]/g, "");
  if (digits === "") return null;
  if (!/^\d+$/.test(digits)) return undefined;
  return Number(digits);
}

const compact = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});

/** Annual revenue like $1.2M. */
export function formatRevenue(revenue: number | null) {
  if (revenue === null) return null;
  return `$${compact.format(revenue)}`;
}

/** Number of employees like 1.2K. */
export function formatSize(size: number | null) {
  if (size === null) return null;
  return compact.format(size);
}
