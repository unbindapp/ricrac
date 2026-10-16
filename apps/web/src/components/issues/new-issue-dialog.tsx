import { useNavigate } from "@tanstack/react-router";
import { ChevronDownIcon, CornerLeftUpIcon, XIcon } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { useLocalStorage } from "usehooks-ts";

import {
  applyPatch,
  draftToCreateArgs,
  EMPTY_DRAFT,
  type TIssueDraft,
} from "@/components/issues/issue-draft";
import { IssueProperties } from "@/components/issues/issue-properties";
import { type TNewIssueDefaults, useNewIssueStore } from "@/components/issues/new-issue-store";
import { useTeamDetail, useWorkspaceUsers } from "@/components/issues/picker-sources";
import { PropertyPicker } from "@/components/issues/property-picker";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { NoTeamsNotice } from "@/components/shared/no-teams-notice";
import { TeamIcon } from "@/components/shared/team-icon";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/toast";
import { useTeams } from "@/components/workspace/teams-provider";
import { useTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { ISSUE_TITLE_MAX_LENGTH, useCreateIssueMutation } from "@/lib/queries/issues";
import { useUploadImage } from "@/lib/queries/uploads";

const LAST_TEAM_KEY = "new-issue-team";

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

function useInitialTeamId(defaults: TNewIssueDefaults) {
  const { teams, myTeams } = useTeams();
  const routeTeam = useTeamFromRoute();
  const [lastTeamId] = useLocalStorage<string | null>(LAST_TEAM_KEY, null);
  const candidates = [defaults.teamId, routeTeam?.id, lastTeamId, myTeams[0]?.id, teams[0]?.id];
  return candidates.find((id) => id && teams.some((team) => team.id === id)) ?? undefined;
}

/** Properties that only make sense in the team they were picked for. */
const TEAM_SCOPED: Partial<TIssueDraft> = {
  state: undefined,
  cycle: null,
  estimate: null,
  labels: [],
};

function NewIssueForm({ defaults, onClose }: { defaults: TNewIssueDefaults; onClose: () => void }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const initialTeamId = useInitialTeamId(defaults);
  const [teamId, setTeamId] = useState<string | undefined>(initialTeamId);
  const [, setLastTeamId] = useLocalStorage<string | null>(LAST_TEAM_KEY, null);
  const [patch, setPatch] = useState<Partial<TIssueDraft>>({});
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [createMore, setCreateMore] = useState(false);
  const [formKey, setFormKey] = useState(0);
  const titleRef = useRef<HTMLInputElement>(null);
  const submittingRef = useRef(false);
  const createMoreId = useId();
  const { teams, myTeams, isPending: isTeamsPending } = useTeams();
  const team = useTeamDetail(teamId);
  const { users } = useWorkspaceUsers();
  const uploadImage = useUploadImage(workspace.id);
  const { mutateAsync: createIssue, isPending } = useCreateIssueMutation(workspace.id);

  const provided = Object.fromEntries(
    Object.entries(defaults.draft ?? {}).filter(([, value]) => value !== undefined),
  );
  const base: TIssueDraft = {
    ...EMPTY_DRAFT,
    ...provided,
    ...(teamId === initialTeamId ? {} : TEAM_SCOPED),
  };
  base.state ??= team?.defaultIssueState ?? undefined;
  const draft = applyPatch(base, patch);
  const pickableTeams = myTeams.length > 0 ? myTeams : teams;
  const teamName = teams.find((item) => item.id === teamId)?.name;

  useEffect(() => {
    titleRef.current?.focus();
  }, [formKey]);

  const submit = async () => {
    if (!teamId || !title.trim() || submittingRef.current) return;
    submittingRef.current = true;
    const created = await createIssue(draftToCreateArgs({ teamId, title, description, draft }))
      .catch(() => null)
      .finally(() => {
        submittingRef.current = false;
      });
    if (!created) return;
    setLastTeamId(teamId);
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

  const changeTeam = (nextTeamId: string) => {
    if (nextTeamId === teamId) return;
    setTeamId(nextTeamId);
    setPatch((current) => ({ ...current, ...TEAM_SCOPED }));
  };

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <div className="flex min-w-0 items-center gap-2 pr-8">
        <PropertyPicker
          label="Team"
          options={pickableTeams.map((item) => ({
            value: item.id,
            label: item.name,
            description: item.key,
            keywords: [item.key],
            icon: <TeamIcon teamKey={item.key} icon={item.icon} color={item.color} size="sm" />,
          }))}
          value={teamId ? [teamId] : []}
          onChange={([id]) => changeTeam(id)}
          trigger={
            <Button
              variant="outline"
              size="sm"
              aria-label={`Team: ${teamName ?? "none"}, change team`}
              className="max-w-48"
            >
              {team && (
                <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />
              )}
              <span className="truncate">{teamName ?? "Pick a team"}</span>
              <ChevronDownIcon className="text-muted-foreground size-3.5" />
            </Button>
          }
        />
        <DialogTitle className="truncate">
          {draft.parent ? "New sub-issue" : "New issue"}
        </DialogTitle>
      </div>
      {!isTeamsPending && teams.length === 0 && (
        <NoTeamsNotice entity="Issues" onNavigate={onClose} />
      )}
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
        onUploadImage={uploadImage}
      />
      {teamId && (
        <IssueProperties
          teamId={teamId}
          layout="inline"
          value={draft}
          omit={["parent"]}
          onChange={(next) => setPatch((current) => ({ ...current, ...next }))}
        />
      )}
      <DialogFooter className="items-center border-t pt-4">
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
        <Button type="submit" size="sm" isPending={isPending} disabled={!teamId || !title.trim()}>
          Create issue
          <Kbd className="bg-primary-foreground/2-10 text-primary-foreground border-transparent">
            ⌘↵
          </Kbd>
        </Button>
      </DialogFooter>
    </form>
  );
}
