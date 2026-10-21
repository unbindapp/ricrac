export type TMediaKind = "image" | "video";

const videoExtensions: Record<string, string> = {
  "video/mp4": ".mp4",
  "video/webm": ".webm",
  "video/quicktime": ".mov",
  "video/x-m4v": ".m4v",
  "video/ogg": ".ogv",
};

const VIDEO_PATTERN = /\.(mp4|webm|mov|m4v|ogv)$/i;
const IMAGE_PATTERN = /\.(png|jpe?g|gif|webp|svg|avif|bmp)$/i;

function pathOf(src: string) {
  try {
    return new URL(src, "http://local").pathname;
  } catch {
    return src;
  }
}

/** An embed `![name](src)` plays as a video when its name or path ends in a video extension. */
export function embedKind(
  name: string | null | undefined,
  src: string | null | undefined,
): TMediaKind {
  if (VIDEO_PATTERN.test(name ?? "") || VIDEO_PATTERN.test(pathOf(src ?? ""))) return "video";
  return "image";
}

const EMBED_PATTERN = /!\[([^\]]*)\]\(<?([^)\s>]+)>?(?:\s+"[^"]*")?\)/;

/** The first image or video embedded in the Markdown, without its layout fragment. */
export function firstEmbed(markdown: string): { url: string; kind: TMediaKind } | null {
  const match = EMBED_PATTERN.exec(markdown);
  if (!match) return null;
  const [, name, src] = match;
  const { url } = mediaLayout(src);
  return { url, kind: embedKind(name, url) };
}

export function fileKind(name: string): TMediaKind | null {
  if (VIDEO_PATTERN.test(name)) return "video";
  if (IMAGE_PATTERN.test(name)) return "image";
  return null;
}

export function uploadKind(file: { type: string }): TMediaKind | null {
  if (file.type.startsWith("image/")) return "image";
  if (file.type in videoExtensions) return "video";
  return null;
}

/** The name to embed a file under: videos keep a video extension so they play. */
export function embedName(file: { name: string; type: string }) {
  const name = file.name.replace(/[[\]]/g, "");
  const extension = videoExtensions[file.type];
  if (!extension || VIDEO_PATTERN.test(name)) return name;
  return `${name}${extension}`;
}

export const mediaAccept = ["image/*", ...Object.keys(videoExtensions)].join(",");

export type TMediaAlign = "left" | "center" | "right";

export type TMediaLayout = { width: number | null; align: TMediaAlign | null };

const alignments: TMediaAlign[] = ["left", "center", "right"];

/** Width and alignment ride in the embed's URL fragment, which servers never see. */
export function mediaLayout(src: string): { url: string; layout: TMediaLayout } {
  const hashAt = src.indexOf("#");
  const layout: TMediaLayout = { width: null, align: null };
  if (hashAt < 0) return { url: src, layout };
  const params = new URLSearchParams(src.slice(hashAt + 1));
  const width = Number(params.get("width"));
  const align = params.get("align") as TMediaAlign | null;
  if (!params.has("width") && !params.has("align")) return { url: src, layout };
  if (Number.isFinite(width) && width > 0) layout.width = Math.round(width);
  if (align && alignments.includes(align)) layout.align = align;
  return { url: src.slice(0, hashAt), layout };
}

export function withMediaLayout(src: string, change: Partial<TMediaLayout>) {
  const { url, layout } = mediaLayout(src);
  const next = { ...layout, ...change };
  const params = new URLSearchParams();
  if (next.width) params.set("width", String(Math.round(next.width)));
  if (next.align && next.align !== "left") params.set("align", next.align);
  const fragment = params.toString();
  return fragment ? `${url}#${fragment}` : url;
}
