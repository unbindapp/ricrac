import type { Editor } from "@tiptap/core";
import type { Node as PMNode } from "@tiptap/pm/model";
import { NodeSelection } from "@tiptap/pm/state";
import { useSyncExternalStore } from "react";

import { toast } from "@/components/ui/toast";
import { embedName, type TMediaKind, uploadKind } from "@/lib/helpers/media";
import type { TUploadProgress } from "@/lib/queries/uploads";
import { errorText } from "@/lib/server/errors";

export type TEditorUpload = (file: File, onProgress?: TUploadProgress) => Promise<string>;

type TPendingUpload = {
  name: string;
  kind: TMediaKind | null;
  progress: number;
  previewUrl: string | null;
};

const pending = new Map<string, TPendingUpload>();
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function update(id: string, change: Partial<TPendingUpload>) {
  const current = pending.get(id);
  if (!current) return;
  pending.set(id, { ...current, ...change });
  emit();
}

function finish(id: string) {
  const current = pending.get(id);
  if (current?.previewUrl) URL.revokeObjectURL(current.previewUrl);
  pending.delete(id);
  emit();
}

export function usePendingUpload(id: string) {
  return useSyncExternalStore(subscribe, () => pending.get(id));
}

export function hasPendingUploads() {
  return pending.size > 0;
}

export function editorHasPendingUploads(editor: Editor) {
  let found = false;
  editor.state.doc.descendants((node) => {
    if (found) return false;
    if (node.attrs.uploadId) found = true;
    return !found;
  });
  return found;
}

function findUpload(editor: Editor, id: string) {
  let found: { node: PMNode; pos: number } | null = null;
  editor.state.doc.descendants((node, pos) => {
    if (found) return false;
    if (node.attrs.uploadId === id) found = { node, pos };
    return !found;
  });
  return found as { node: PMNode; pos: number } | null;
}

/** Placeholders are left out of the Markdown until their upload lands. */
export function insertUploads(editor: Editor, files: File[], upload: TEditorUpload, at?: number) {
  if (files.length === 0 || !editor.isEditable) return;
  const placeholders = files.map((file) => {
    const id = crypto.randomUUID();
    const kind = uploadKind(file);
    pending.set(id, {
      name: file.name,
      kind,
      progress: 0,
      previewUrl: kind ? URL.createObjectURL(file) : null,
    });
    return { file, id, kind };
  });
  emit();
  const nodes = placeholders.map(({ file, id }) => ({
    type: "image",
    attrs: { src: "", alt: embedName(file), uploadId: id },
  }));
  editor.chain().focus().insertContentAt(insertionPoint(editor, at), nodes).run();
  moveIntoTextAfter(editor, placeholders[placeholders.length - 1].id);
  for (const placeholder of placeholders) void send(editor, placeholder, upload);
}

function insertionPoint(editor: Editor, at: number | undefined) {
  const { selection } = editor.state;
  if (at !== undefined) return at;
  if (selection instanceof NodeSelection) return selection.to;
  const { $from } = selection;
  if ($from.parent.isTextblock && $from.parent.content.size === 0 && $from.depth > 0) {
    return { from: $from.before(), to: $from.after() };
  }
  return selection.from;
}

function moveIntoTextAfter(editor: Editor, lastId: string) {
  const last = findUpload(editor, lastId);
  if (!last) return;
  const end = last.pos + last.node.nodeSize;
  if (editor.state.doc.nodeAt(end)?.isTextblock) {
    editor.commands.setTextSelection(end + 1);
    return;
  }
  editor
    .chain()
    .insertContentAt(end, { type: "paragraph" })
    .setTextSelection(end + 1)
    .run();
}

async function send(
  editor: Editor,
  { file, id, kind }: { file: File; id: string; kind: TMediaKind | null },
  upload: TEditorUpload,
) {
  try {
    const url = await upload(file, (progress) => update(id, { progress }));
    if (editor.isDestroyed) return;
    const found = findUpload(editor, id);
    if (!found) return;
    const { node, pos } = found;
    const { schema, tr } = editor.state;
    if (kind) {
      editor.view.dispatch(
        tr.setNodeMarkup(pos, undefined, { ...node.attrs, src: url, uploadId: null }),
      );
      return;
    }
    const link = schema.marks.link.create({ href: url });
    const paragraph = schema.nodes.paragraph.create(null, schema.text(file.name, [link]));
    editor.view.dispatch(tr.replaceWith(pos, pos + node.nodeSize, paragraph));
  } catch (error) {
    toast.add({
      type: "error",
      title: `Couldn't upload ${file.name}`,
      description: errorText(error),
    });
    if (editor.isDestroyed) return;
    const found = findUpload(editor, id);
    if (found)
      editor.view.dispatch(editor.state.tr.delete(found.pos, found.pos + found.node.nodeSize));
  } finally {
    finish(id);
  }
}
