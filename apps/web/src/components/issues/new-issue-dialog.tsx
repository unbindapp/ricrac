import { formatForDisplay } from "@tanstack/react-hotkeys";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useId, useRef, useState } from "react";

import { CornerLeftUpIcon, XIcon } from "@/components/icons";
import {
  applyPatch,
  draftToCreateArgs,
  EMPTY_DRAFT,
  type TIssueDraft,
} from "@/components/issues/issue-draft";
import { GithubRepositoryPicker } from "@/components/issues/github-repository-picker";
import { IssueProperties } from "@/components/issues/issue-properties";
import { type TNewIssueDefaults, useNewIssueStore } from "@/components/issues/new-issue-store";
import { useTeamDetail, useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/toast";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { ISSUE_TITLE_MAX_LENGTH, useCreateIssueMutation } from "@/lib/queries/issues";
import { useEditorUploads } from "@/lib/queries/uploads";

export function NewIssueDialog() {
  const isOpen = useNewIssueStore((state) => state.isOpen);
  const close = useNewIssueStore((state) => state.close);
  const defaults = useNewIssueStore((state) => state.defaults);
  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && close()}>
      <DialogContent className="w-2xl max-w-full">
        {isOpen && <NewIssueForm defaults={defaults} onClose={close} />}
      </DialogContent>
    </Dialog>
  );
}

function NewIssueForm({ defaults, onClose }: { defaults: TNewIssueDefaults; onClose: () => void }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const teamId = useWorkspaceTeam()?.id;
  const [patch, setPatch] = useState<Partial<TIssueDraft>>({});
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [githubRepository, setGithubRepository] = useState<string>();
  const [createMore, setCreateMore] = useState(false);
  const [formKey, setFormKey] = useState(0);
  const titleRef = useRef<HTMLInputElement>(null);
  const submittingRef = useRef(false);
  const createMoreId = useId();
  const team = useTeamDetail(teamId);
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: createIssue, isPending } = useCreateIssueMutation(workspace.id);

  const provided = Object.fromEntries(
    Object.entries(defaults.draft ?? {}).filter(([, value]) => value !== undefined),
  );
  const base: TIssueDraft = { ...EMPTY_DRAFT, ...provided };
  base.state ??= team?.defaultIssueState ?? undefined;
  const draft = applyPatch(base, patch);

  useEffect(() => {
    titleRef.current?.focus();
  }, [formKey]);

  const submit = async () => {
    if (!teamId || !title.trim() || submittingRef.current || uploads.isUploading) return;
    submittingRef.current = true;
    const args = draftToCreateArgs({ teamId, title, description, draft });
    const created = await createIssue(githubRepository ? { ...args, githubRepository } : args)
      .catch(() => null)
      .finally(() => {
        submittingRef.current = false;
      });
    if (!created) return;
    toast.add({
      type: "success",
      title: `Created ${created.identifier}`,
      description: created.title,
      data: {
        action: (
          <Button
            variant="outline"
            size="sm"
            onClick={() =>
              void navigate({
                to: "/$workspace/issue/$identifier",
                params: { workspace: workspace.urlKey, identifier: created.identifier },
              })
            }
          >
            Open {created.identifier}
          </Button>
        ),
      },
    });
    if (!createMore) {
      onClose();
      return;
    }
    setTitle("");
    setDescription("");
    setFormKey((key) => key + 1);
  };

  useHotkeyScope([{ hotkey: "Mod+Enter", callback: () => void submit(), ignoreInputs: false }], {
    modal: true,
  });

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <DialogTitle className="truncate pr-8">
        {draft.parent ? "New sub-issue" : "New issue"}
      </DialogTitle>
      {draft.parent && (
        <div className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-sm">
          <CornerLeftUpIcon className="size-4 shrink-0" />
          <span className="shrink-0">Sub-issue of</span>
          <span className="font-mono text-xs">{draft.parent.identifier}</span>
          <span className="text-foreground truncate">{draft.parent.title}</span>
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Remove parent"
            className="size-6"
            onClick={() => setPatch((current) => ({ ...current, parent: null }))}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      )}
      <Input
        ref={titleRef}
        framed
        aria-label="Issue title"
        placeholder="Issue title"
        maxLength={ISSUE_TITLE_MAX_LENGTH}
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        className="px-0 text-base font-medium"
      />
      <MarkdownEditor
        key={formKey}
        label="Description"
        value={description}
        onChange={setDescription}
        placeholder="Add a description…"
        users={users}
        {...uploads.editorProps}
      />
      {teamId && (
        <IssueProperties
          teamId={teamId}
          layout="inline"
          value={draft}
          omit={["parent"]}
          onChange={(next) => setPatch((current) => ({ ...current, ...next }))}
          extraChips={
            <GithubRepositoryPicker
              teamId={teamId}
              projectId={draft.project?.id}
              value={githubRepository}
              onChange={setGithubRepository}
            />
          }
        />
      )}
      <DialogFooter className="dank:border-t-0 dank:pt-0 items-center border-t pt-4">
        <label
          htmlFor={createMoreId}
          className="text-muted-foreground mr-auto flex items-center gap-2 text-sm"
        >
          <Switch id={createMoreId} checked={createMore} onCheckedChange={setCreateMore} />
          Create more
        </label>
        <Button type="button" variant="outline" size="sm" onClick={onClose}>
          Cancel
        </Button>
        <Button
          type="submit"
          variant="default"
          size="sm"
          isPending={isPending}
          disabled={!teamId || !title.trim() || uploads.isUploading}
        >
          Create issue
          <Kbd className="bg-primary-foreground/2-10 text-primary-foreground group-disabled/button:text-muted-foreground dank:bg-m3-primary/10 dank:text-m3-primary dank:group-disabled/button:text-m3-on-surface/38 border-transparent group-disabled/button:bg-transparent">
            {formatForDisplay("Mod+Enter")}
          </Kbd>
        </Button>
      </DialogFooter>
    </form>
  );
}
