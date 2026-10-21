import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { DiscussionDetail, IssueRef } from "@/api/ops.gen";
import { IssueLinkPicker } from "@/components/forum/issue-link-picker";
import { THREAD_KINDS, threadKinds } from "@/components/forum/thread-kinds";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import {
  DISCUSSION_TITLE_MAX_LENGTH,
  useSaveDiscussionThreadMutation,
} from "@/lib/queries/discussions";
import { useEditorUploads } from "@/lib/queries/uploads";

const schema = z.object({
  kind: z.enum(THREAD_KINDS),
  title: z
    .string()
    .trim()
    .min(1, "Give it a title.")
    .max(DISCUSSION_TITLE_MAX_LENGTH, "Keep the title shorter."),
});

const kindItems = THREAD_KINDS.map((kind) => {
  const { label, icon: Icon } = threadKinds[kind];
  return { value: kind, label, icon: <Icon className="text-muted-foreground size-4" /> };
});

type TProps = { discussion: DiscussionDetail; trigger: ReactElement };

export function NewThreadDialog({ discussion, trigger }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-2xl max-w-full">
        <DialogHeader>
          <DialogTitle>New thread</DialogTitle>
        </DialogHeader>
        {isOpen && <NewThreadForm discussion={discussion} onCreated={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function NewThreadForm({
  discussion,
  onCreated,
}: {
  discussion: DiscussionDetail;
  onCreated: () => void;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: save } = useSaveDiscussionThreadMutation(workspace.id);
  const [body, setBody] = useState("");
  const [issue, setIssue] = useState<IssueRef | null>(null);
  const form = useAppForm({
    defaultValues: { kind: "general", title: "" },
    validators: { onSubmit: schema },
    onSubmit: async ({ value }) => {
      const { kind, title } = schema.parse(value);
      const thread = await save({
        discussion: discussion.id,
        kind,
        title,
        body,
        issue: issue?.id,
      }).catch(() => null);
      if (!thread) return;
      onCreated();
      await navigate({
        to: "/$workspace/discussion/$slug",
        params: { workspace: workspace.urlKey, slug: entitySlug(discussion) },
        search: { thread: thread.id },
        replace: true,
      });
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <div className="grid gap-4 sm:grid-cols-3">
        <form.AppField
          name="kind"
          children={(field) => <field.SelectField label="Kind" items={kindItems} />}
        />
        <div className="sm:col-span-2">
          <form.AppField
            name="title"
            children={(field) => (
              <field.TextField label="Title" placeholder="What is this thread about?" autoFocus />
            )}
          />
        </div>
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium">Body</span>
        <MarkdownEditor
          label="Body"
          value={body}
          onChange={setBody}
          placeholder="Add the details…"
          users={users}
          {...uploads.editorProps}
          framed
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium">Tracking issue</span>
        <div className="flex">
          <IssueLinkPicker issue={issue} onChange={setIssue} />
        </div>
      </div>
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm" disabled={uploads.isUploading}>
            Start thread
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}
