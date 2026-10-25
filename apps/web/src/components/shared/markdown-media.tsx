import { type ReactNode, useState } from "react";

import { DownloadIcon, ExternalLinkIcon, PlayIcon, XIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { embedKind, mediaLayout, type TMediaKind, type TMediaLayout } from "@/lib/helpers/media";
import { useUiTheme } from "@/themes/ui-theme-provider";

export function MarkdownMedia({ src, alt }: { src: string; alt?: string | null }) {
  const { mediaViewer } = useUiTheme();
  const [isOpen, setIsOpen] = useState(false);
  const { url, layout } = mediaLayout(src);
  const isVideo = embedKind(alt, url) === "video";
  const name = alt || (isVideo ? "Video" : "Image");
  if (isVideo && mediaViewer === "dialog") {
    return (
      <MediaFrame layout={layout}>
        <MediaVideo src={url} alt={alt} />
      </MediaFrame>
    );
  }
  if (isVideo) {
    return (
      <MediaFrame layout={layout}>
        <Button
          variant="ghost"
          forceMinSize={false}
          aria-label={`Play ${name}`}
          className="relative block h-auto w-full p-0"
          onClick={() => setIsOpen(true)}
        >
          <video src={url} muted playsInline preload="metadata" className="rr-media bg-muted" />
          <span className="bg-m3-scrim/50 text-m3-inverse-on-surface absolute top-1/2 left-1/2 flex size-14 -translate-1/2 items-center justify-center rounded-full">
            <PlayIcon className="size-8 fill-current" />
          </span>
        </Button>
        <MediaLightbox src={url} name={name} kind="video" open={isOpen} onOpenChange={setIsOpen} />
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
  kind = "image",
  open,
  onOpenChange,
}: {
  src: string;
  name: string;
  kind?: TMediaKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { mediaViewer } = useUiTheme();
  if (mediaViewer === "stage") {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent variant="styleless" className="items-center gap-3">
          <div className="bg-m3-surface-container-high text-m3-on-surface shadow-elevation-2 flex max-w-full items-center gap-1 self-center rounded-full py-1 pr-1 pl-4">
            <DialogTitle className="type-title-small min-w-0 flex-1 truncate">{name}</DialogTitle>
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
            <DialogClose render={<Button variant="ghost" size="icon" aria-label="Close" />}>
              <XIcon className="size-4" />
            </DialogClose>
          </div>
          {kind === "video" ? (
            <video
              src={src}
              title={name}
              controls
              autoPlay
              playsInline
              className="rr-stage-media"
            />
          ) : (
            <img src={src} alt={name} className="rr-stage-media" />
          )}
        </DialogContent>
      </Dialog>
    );
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-dvh w-fit max-w-dvw"
        headerActions={
          <>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Open the original"
              className="text-muted-foreground"
              render={<a href={src} target="_blank" rel="noreferrer noopener" />}
            >
              <ExternalLinkIcon className="size-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Download"
              className="text-muted-foreground"
              render={<a href={src} download={name} />}
            >
              <DownloadIcon className="size-4" />
            </Button>
          </>
        }
      >
        <DialogTitle className="truncate pr-28 text-sm">{name}</DialogTitle>
        <img src={src} alt={name} className="rr-lightbox-image" />
      </DialogContent>
    </Dialog>
  );
}
