import { useEffect, useRef, useState } from "react";

import { FileIcon, PlayIcon, XIcon } from "@/components/icons";
import { UploadBar } from "@/components/shared/editor/media-node";
import { MarkdownMedia } from "@/components/shared/markdown-media";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { embedName, type TMediaKind, uploadKind } from "@/lib/helpers/media";
import type { TChatAttachment } from "@/lib/helpers/chat";
import { useUploadAsset } from "@/lib/queries/uploads";
import { errorText } from "@/lib/server/errors";

export type TDraftAttachment = {
  id: string;
  name: string;
  kind: TMediaKind | "file";
  previewUrl: string | null;
  progress: number;
  /** Set once the file is uploaded. */
  markdown: string | null;
};

function draftOf(attachment: TChatAttachment): TDraftAttachment {
  return {
    id: crypto.randomUUID(),
    name: attachment.name,
    kind: attachment.kind,
    previewUrl: attachment.kind === "file" ? null : attachment.url,
    progress: 1,
    markdown: attachment.markdown,
  };
}

function markdownOf(file: File, url: string) {
  if (uploadKind(file)) return `![${embedName(file)}](${url})`;
  return `[${file.name.replace(/[[\]]/g, "")}](${url})`;
}

/** Files a message carries apart from its text, uploaded as soon as they are added. */
export function useDraftAttachments(workspaceId: string, initial: TChatAttachment[] = []) {
  const upload = useUploadAsset(workspaceId);
  const [items, setItems] = useState(() => initial.map(draftOf));
  const localUrls = useRef(new Set<string>());

  useEffect(() => {
    const urls = localUrls.current;
    return () => urls.forEach((url) => URL.revokeObjectURL(url));
  }, []);

  const change = (id: string, next: Partial<TDraftAttachment>) =>
    setItems((current) => current.map((item) => (item.id === id ? { ...item, ...next } : item)));

  const remove = (id: string) =>
    setItems((current) => {
      const removed = current.find((item) => item.id === id);
      if (removed?.previewUrl && localUrls.current.delete(removed.previewUrl)) {
        URL.revokeObjectURL(removed.previewUrl);
      }
      return current.filter((item) => item.id !== id);
    });

  const send = async (id: string, file: File) => {
    try {
      const url = await upload(file, (progress) => change(id, { progress }));
      change(id, { progress: 1, markdown: markdownOf(file, url) });
    } catch (error) {
      toast.add({
        type: "error",
        title: `Couldn't upload ${file.name}`,
        description: errorText(error),
      });
      remove(id);
    }
  };

  const add = (files: File[]) => {
    const added = files.map((file) => {
      const kind = uploadKind(file);
      const previewUrl = kind ? URL.createObjectURL(file) : null;
      if (previewUrl) localUrls.current.add(previewUrl);
      const item: TDraftAttachment = {
        id: crypto.randomUUID(),
        name: file.name,
        kind: kind ?? "file",
        previewUrl,
        progress: 0,
        markdown: null,
      };
      return { item, file };
    });
    setItems((current) => [...current, ...added.map(({ item }) => item)]);
    for (const { item, file } of added) void send(item.id, file);
  };

  const clear = () => {
    localUrls.current.forEach((url) => URL.revokeObjectURL(url));
    localUrls.current.clear();
    setItems([]);
  };

  return {
    items,
    add,
    remove,
    clear,
    isUploading: items.some((item) => item.markdown === null),
    uploaded: items.flatMap((item) => (item.markdown ? [{ markdown: item.markdown }] : [])),
  };
}

export function AttachmentTray({
  items,
  onRemove,
}: {
  items: TDraftAttachment[];
  onRemove: (id: string) => void;
}) {
  if (items.length === 0) return null;
  return (
    <ul aria-label="Attachments" className="flex flex-wrap gap-2">
      {items.map((item) => (
        <li
          key={item.id}
          className="bg-card relative flex size-24 flex-col items-center justify-center gap-1.5 overflow-hidden rounded-md border"
        >
          <AttachmentPreview item={item} />
          {item.markdown === null && (
            <span className="absolute inset-x-2 bottom-2">
              <UploadBar percent={Math.round(item.progress * 100)} />
            </span>
          )}
          <Button
            variant="secondary"
            size="icon"
            forceMinSize={false}
            aria-label={`Remove ${item.name}`}
            className="dank:size-8 dank:[&>svg]:size-5 absolute top-1 right-1 size-6"
            onClick={() => onRemove(item.id)}
          >
            <XIcon className="size-3.5" />
          </Button>
        </li>
      ))}
    </ul>
  );
}

function AttachmentPreview({ item }: { item: TDraftAttachment }) {
  if (item.kind === "image" && item.previewUrl) {
    return <img src={item.previewUrl} alt={item.name} className="size-full object-cover" />;
  }
  if (item.kind === "video" && item.previewUrl) {
    return (
      <>
        <video src={item.previewUrl} muted playsInline className="size-full object-cover" />
        <PlayIcon className="absolute size-5 fill-current text-white" />
      </>
    );
  }
  return (
    <>
      <FileIcon className="text-muted-foreground size-5" />
      <span className="w-full truncate px-2 text-center text-xs">{item.name}</span>
    </>
  );
}

/** What a message carries under its text: media to open, files to download. */
export function MessageAttachments({ attachments }: { attachments: TChatAttachment[] }) {
  if (attachments.length === 0) return null;
  return (
    <div className="rr-chat flex flex-wrap items-start gap-2">
      {attachments.map((attachment, index) =>
        attachment.kind === "file" ? (
          <a
            key={`${index}:${attachment.markdown}`}
            href={attachment.url}
            target="_blank"
            rel="noreferrer noopener"
            className="bg-card has-hover:hover:bg-card-hover dank:relative dank:state-layer dank:has-hover:hover:bg-card dank:px-4 flex h-14 w-64 max-w-full items-center gap-3 rounded-md border px-3 transition-colors"
          >
            <FileIcon className="text-muted-foreground size-5 shrink-0" />
            <span className="text-primary min-w-0 truncate text-sm">{attachment.name}</span>
          </a>
        ) : (
          <MarkdownMedia
            key={`${index}:${attachment.markdown}`}
            src={attachment.url}
            alt={attachment.name}
          />
        ),
      )}
    </div>
  );
}
