import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import { DiscussionTagsPicker } from "@/components/forum/discussion-tags";
import { ThumbnailPicker } from "@/components/forum/thumbnail-picker";
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
import { DISCUSSION_TITLE_MAX_LENGTH, useSaveDiscussionMutation } from "@/lib/queries/discussions";
import { useEditorUploads } from "@/lib/queries/uploads";

const schema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "Give it a title.")
    .max(DISCUSSION_TITLE_MAX_LENGTH, "Keep the title shorter."),
});

type TProps = {
  trigger: ReactElement;
  defaultTeamId?: string | null;
};

export function NewDiscussionDialog({ trigger }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-2xl max-w-full">
        <DialogHeader>
          <DialogTitle>New discussion</DialogTitle>
        </DialogHeader>
        {isOpen && <NewDiscussionForm />}
      </DialogContent>
    </Dialog>
  );
}

function NewDiscussionForm() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: save } = useSaveDiscussionMutation(workspace.id);
  const [body, setBody] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [thumbnailUrl, setThumbnailUrl] = useState<string | null>(null);
  const form = useAppForm({
    defaultValues: { title: "" },
    validators: { onSubmit: schema },
    onSubmit: async ({ value }) => {
      const discussion = await save({
        title: schema.parse(value).title,
        body,
        tags,
        thumbnailUrl,
      }).catch(() => null);
      if (!discussion) return;
      await navigate({
        to: "/$workspace/discussion/$slug",
        params: { workspace: workspace.urlKey, slug: entitySlug(discussion) },
      });
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="title"
        children={(field) => (
          <field.TextField label="Title" placeholder="What do you want to talk about?" autoFocus />
        )}
      />
      <div className="flex min-w-0 flex-col gap-1.5">
        <span className="text-sm font-medium">Tags</span>
        <div className="flex">
          <DiscussionTagsPicker value={tags} onChange={setTags} />
        </div>
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium">Body</span>
        <MarkdownEditor
          label="Body"
          value={body}
          onChange={setBody}
          placeholder="Write it up…"
          users={users}
          {...uploads.editorProps}
          framed
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium">Thumbnail</span>
        <ThumbnailPicker value={thumbnailUrl} onChange={setThumbnailUrl} body={body} />
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
            Start discussion
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}
