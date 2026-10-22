import { type ReactNode, useState } from "react";
import { DownloadIcon, ExternalLinkIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { embedKind, mediaLayout, type TMediaLayout } from "@/lib/helpers/media";

export function MarkdownMedia({ src, alt }: { src: string; alt?: string | null }) {
  const [isOpen, setIsOpen] = useState(false);
  const { url, layout } = mediaLayout(src);
  const name = alt || "Image";
  if (embedKind(alt, url) === "video") {
    return (
      <MediaFrame layout={layout}>
        <MediaVideo src={url} alt={alt} />
      </MediaFrame>
    );
  }
  return (
    <MediaFrame layout={layout}>
      <Button
        variant="ghost"
        forceMinSize={false}
        aria-label={`Open ${name}`}
        className="block h-auto w-full cursor-zoom-in p-0"
        onClick={() => setIsOpen(true)}
      >
        <img src={url} alt={alt ?? ""} loading="lazy" className="rr-media" />
      </Button>
      <MediaLightbox src={url} name={name} open={isOpen} onOpenChange={setIsOpen} />
    </MediaFrame>
  );
}

export function MediaFrame({
  layout,
  isSelected,
  children,
}: {
  layout: TMediaLayout;
  isSelected?: boolean;
  children: ReactNode;
}) {
  return (
    <span
      data-slot="media-frame"
      data-align={layout.align ?? "left"}
      data-selected={isSelected || undefined}
      className="rr-embed group/media"
      style={layout.width ? { width: layout.width } : undefined}
    >
      {children}
    </span>
  );
}

export function MediaVideo({ src, alt }: { src: string; alt?: string | null }) {
  return (
    <video
      src={src}
      title={alt ?? undefined}
      controls
      playsInline
      preload="metadata"
      className="rr-media bg-muted"
    />
  );
}

export function MediaLightbox({
  src,
  name,
  open,
  onOpenChange,
}: {
  src: string;
  name: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-dvh w-fit max-w-dvw">
        <div className="flex items-center gap-2 pr-8">
          <DialogTitle className="min-w-0 flex-1 truncate pr-0 text-sm">{name}</DialogTitle>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Open the original"
            render={<a href={src} target="_blank" rel="noreferrer noopener" />}
          >
            <ExternalLinkIcon className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Download"
            render={<a href={src} download={name} />}
          >
            <DownloadIcon className="size-4" />
          </Button>
        </div>
        <img src={src} alt={name} className="rr-lightbox-image" />
      </DialogContent>
    </Dialog>
  );
}
