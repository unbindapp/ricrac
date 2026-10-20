import { Image } from "@tiptap/extension-image";
import { NodeSelection } from "@tiptap/pm/state";
import { NodeViewWrapper, ReactNodeViewRenderer, type ReactNodeViewProps } from "@tiptap/react";
import {
  AlignCenterIcon,
  AlignLeftIcon,
  AlignRightIcon,
  DownloadIcon,
  GripVerticalIcon,
  Maximize2Icon,
  PaperclipIcon,
  Trash2Icon,
} from "lucide-react";
import { type DragEvent, type PointerEvent, useRef, useState } from "react";

import { usePendingUpload } from "@/components/shared/editor/uploads";
import { MediaFrame, MediaLightbox, MediaVideo } from "@/components/shared/markdown-media";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { cn } from "@/components/ui/utils";
import {
  embedKind,
  mediaLayout,
  type TMediaAlign,
  type TMediaLayout,
  withMediaLayout,
} from "@/lib/helpers/media";

const MIN_WIDTH = 80;

export const mediaSizes = [
  { label: "S", name: "Small", width: 320 },
  { label: "M", name: "Medium", width: 480 },
  { label: "L", name: "Large", width: 720 },
  { label: "Fit", name: "Original size", width: null },
] as const;

export const mediaAlignments: { value: TMediaAlign; label: string; icon: typeof AlignLeftIcon }[] =
  [
    { value: "left", label: "Align left", icon: AlignLeftIcon },
    { value: "center", label: "Align center", icon: AlignCenterIcon },
    { value: "right", label: "Align right", icon: AlignRightIcon },
  ];

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    media: {
      setMediaLayout: (change: Partial<TMediaLayout>) => ReturnType;
    };
  }
}

/** Images and videos (`![demo.mp4](url)`); a placeholder while one uploads, left out of the Markdown. */
export const EditorMedia = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      uploadId: { default: null, rendered: false },
    };
  },
  renderMarkdown: (node) => {
    if (node.attrs?.uploadId) return "";
    const alt = node.attrs?.alt ?? "";
    const src = node.attrs?.src ?? "";
    const title = node.attrs?.title;
    return title ? `![${alt}](${src} "${title}")` : `![${alt}](${src})`;
  },
  addCommands() {
    return {
      ...this.parent?.(),
      setMediaLayout:
        (change) =>
        ({ state, tr, dispatch }) => {
          const { selection } = state;
          const node = state.doc.nodeAt(selection.from);
          if (node?.type.name !== this.name || !node.attrs.src) return false;
          dispatch?.(
            tr.setNodeMarkup(selection.from, undefined, {
              ...node.attrs,
              src: withMediaLayout(node.attrs.src, change),
            }),
          );
          return true;
        },
    };
  },
  addNodeView() {
    return ReactNodeViewRenderer(MediaNodeView);
  },
}).configure({ inline: false });

function MediaNodeView({
  node,
  selected,
  editor,
  getPos,
  updateAttributes,
  deleteNode,
}: ReactNodeViewProps) {
  const { src, alt, uploadId } = node.attrs as {
    src: string;
    alt: string | null;
    uploadId: string | null;
  };
  if (uploadId) return <UploadingMedia id={uploadId} />;
  return (
    <PlacedMedia
      src={src}
      alt={alt}
      isSelected={selected}
      isEditable={editor.isEditable}
      onChange={(change) => updateAttributes({ src: withMediaLayout(src, change) })}
      onDelete={deleteNode}
      onDragStart={(event) => {
        const pos = getPos();
        if (typeof pos !== "number") return;
        event.stopPropagation();
        const { state, view } = editor;
        const selection = NodeSelection.create(state.doc, pos);
        view.dispatch(state.tr.setSelection(selection));
        view.dragging = { slice: selection.content(), move: true };
        event.dataTransfer.effectAllowed = "copyMove";
        event.dataTransfer.setData("text/plain", alt ?? "");
        const frame = event.currentTarget.closest("[data-slot=media-frame]");
        if (frame) event.dataTransfer.setDragImage(frame, 16, 16);
      }}
    />
  );
}

function PlacedMedia({
  src,
  alt,
  isSelected,
  isEditable,
  onChange,
  onDelete,
  onDragStart,
}: {
  src: string;
  alt: string | null;
  isSelected: boolean;
  isEditable: boolean;
  onChange: (change: Partial<TMediaLayout>) => void;
  onDelete: () => void;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [draftWidth, setDraftWidth] = useState<number | null>(null);
  const frameRef = useRef<HTMLDivElement>(null);
  const { url, layout } = mediaLayout(src);
  const kind = embedKind(alt, url);
  const name = alt || (kind === "video" ? "Video" : "Image");
  const shown = draftWidth ? { ...layout, width: draftWidth } : layout;

  const startResize = (event: PointerEvent<HTMLSpanElement>, side: 1 | -1) => {
    const frame = frameRef.current?.querySelector<HTMLElement>("[data-slot=media-frame]");
    const container = frameRef.current?.parentElement;
    if (!frame || !container) return;
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = frame.getBoundingClientRect().width;
    const maxWidth = container.getBoundingClientRect().width;
    const factor = layout.align === "center" ? 2 : 1;
    let width = startWidth;
    const move = (moveEvent: globalThis.PointerEvent) => {
      const delta = (moveEvent.clientX - startX) * side * factor;
      width = Math.min(maxWidth, Math.max(MIN_WIDTH, startWidth + delta));
      setDraftWidth(width);
    };
    const end = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", end);
      setDraftWidth(null);
      onChange({ width: width >= maxWidth ? null : width });
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", end);
  };

  return (
    <NodeViewWrapper ref={frameRef} data-media={kind} className="rr-embed-block">
      <MediaFrame layout={shown} isSelected={isSelected && isEditable}>
        {kind === "video" ? (
          <MediaVideo src={url} alt={alt} />
        ) : (
          <img
            src={url}
            alt={alt ?? ""}
            className={cn("rr-media", !isEditable && "cursor-zoom-in")}
            data-drag-handle={isEditable || undefined}
            draggable={isEditable}
            onDragStart={onDragStart}
            onClick={() => !isEditable && setIsOpen(true)}
          />
        )}
        {isEditable && (
          <>
            <MediaControls
              kind={kind}
              layout={layout}
              onChange={onChange}
              onOpen={() => setIsOpen(true)}
              onDelete={onDelete}
              onDragStart={onDragStart}
              url={url}
              name={name}
            />
            <span
              aria-hidden
              data-media-resize="start"
              className="rr-media-resize left-0"
              onPointerDown={(event) => startResize(event, -1)}
            />
            <span
              aria-hidden
              data-media-resize="end"
              className="rr-media-resize right-0"
              onPointerDown={(event) => startResize(event, 1)}
            />
          </>
        )}
      </MediaFrame>
      {kind === "image" && (
        <MediaLightbox src={url} name={name} open={isOpen} onOpenChange={setIsOpen} />
      )}
    </NodeViewWrapper>
  );
}

function MediaControls({
  kind,
  layout,
  url,
  name,
  onChange,
  onOpen,
  onDelete,
  onDragStart,
}: {
  kind: "image" | "video";
  layout: TMediaLayout;
  url: string;
  name: string;
  onChange: (change: Partial<TMediaLayout>) => void;
  onOpen: () => void;
  onDelete: () => void;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
}) {
  const align = layout.align ?? "left";
  return (
    <div
      contentEditable={false}
      data-slot="media-controls"
      className="bg-popover text-popover-foreground absolute top-2 right-2 flex items-center rounded-md border p-0.5 opacity-0 shadow-sm transition-opacity group-hover/media:opacity-100 group-data-selected/media:opacity-100 focus-within:opacity-100"
    >
      <SimpleTooltip content="Drag to move">
        <span
          data-drag-handle
          draggable
          aria-label="Drag to move"
          onDragStart={onDragStart}
          className="text-muted-foreground flex size-7 cursor-grab items-center justify-center active:cursor-grabbing"
        >
          <GripVerticalIcon className="size-4" />
        </span>
      </SimpleTooltip>
      {mediaAlignments.map(({ value, label, icon: Icon }) => (
        <ControlButton
          key={value}
          label={label}
          isActive={align === value}
          onClick={() => onChange({ align: value })}
        >
          <Icon className="size-4" />
        </ControlButton>
      ))}
      <span className="bg-border mx-0.5 h-4 w-px" />
      {mediaSizes.map((size) => (
        <ControlButton
          key={size.label}
          label={size.name}
          isActive={layout.width === size.width}
          onClick={() => onChange({ width: size.width })}
        >
          <span className="text-xs font-medium">{size.label}</span>
        </ControlButton>
      ))}
      <span className="bg-border mx-0.5 h-4 w-px" />
      {kind === "image" && (
        <ControlButton label="Open full size" onClick={onOpen}>
          <Maximize2Icon className="size-4" />
        </ControlButton>
      )}
      <SimpleTooltip content="Download">
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label="Download"
          render={<a href={url} download={name} />}
        >
          <DownloadIcon className="size-4" />
        </Button>
      </SimpleTooltip>
      <ControlButton label="Delete" onClick={onDelete}>
        <Trash2Icon className="size-4" />
      </ControlButton>
    </div>
  );
}

function ControlButton({
  label,
  isActive,
  onClick,
  children,
}: {
  label: string;
  isActive?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={label}
        aria-pressed={isActive}
        className={cn(isActive && "bg-accent text-foreground")}
        onMouseDown={(event) => event.preventDefault()}
        onClick={onClick}
      >
        {children}
      </Button>
    </SimpleTooltip>
  );
}

function UploadingMedia({ id }: { id: string }) {
  const upload = usePendingUpload(id);
  const percent = Math.round((upload?.progress ?? 0) * 100);
  const label = `Uploading ${upload?.name ?? "file"}… ${percent}%`;
  return (
    <NodeViewWrapper className="rr-embed-block" data-uploading>
      {upload?.previewUrl ? (
        <span className="rr-embed relative">
          {upload.kind === "video" ? (
            <video src={upload.previewUrl} muted playsInline className="rr-media opacity-60" />
          ) : (
            <img src={upload.previewUrl} alt="" className="rr-media opacity-60" />
          )}
          <span className="bg-popover/90 absolute inset-x-2 bottom-2 flex flex-col gap-1.5 rounded-md border p-2">
            <span className="truncate text-xs">{label}</span>
            <UploadBar percent={percent} />
          </span>
        </span>
      ) : (
        <span className="bg-card inline-flex max-w-full items-center gap-2 rounded-md border px-3 py-2">
          <PaperclipIcon className="text-muted-foreground size-4 shrink-0" />
          <span className="flex min-w-0 flex-col gap-1.5">
            <span className="truncate text-xs">{label}</span>
            <UploadBar percent={percent} />
          </span>
        </span>
      )}
    </NodeViewWrapper>
  );
}

function UploadBar({ percent }: { percent: number }) {
  return (
    <span
      role="progressbar"
      aria-valuenow={percent}
      aria-valuemin={0}
      aria-valuemax={100}
      className="bg-muted block h-1 w-full overflow-hidden rounded-full"
    >
      <span className="bg-primary block h-full transition-all" style={{ width: `${percent}%` }} />
    </span>
  );
}
