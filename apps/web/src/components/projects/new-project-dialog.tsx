import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import type { TeamRef } from "@/api/ops.gen";
import {
  type TNewProjectDefaults,
  useNewProjectStore,
} from "@/components/projects/new-project-store";
import {
  PeoplePicker,
  PersonPicker,
  toUserRef,
  useMentionUsers,
  useWorkspaceUserList,
} from "@/components/projects/people";
import { fromResolution, type TResolution } from "@/components/projects/project-dates";
import {
  PlannedDatePicker,
  ProjectLabelsPicker,
  ProjectPriorityPicker,
  ProjectStatusPicker,
  TeamsPicker,
  useProjectStatuses,
} from "@/components/projects/project-pickers";
import { MarkdownField } from "@/components/shared/markdown-editor";
import { NoTeamsNotice } from "@/components/shared/no-teams-notice";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useTeams } from "@/components/workspace/teams-provider";
import { useTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useEditorUploads } from "@/lib/queries/uploads";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useProjectLabelsById } from "@/components/projects/project-labels";
import { ProjectFormSchema, useSaveProjectMutation } from "@/lib/queries/projects";

export function NewProjectDialog() {
  const isOpen = useNewProjectStore((state) => state.isOpen);
  const close = useNewProjectStore((state) => state.close);
  const defaults = useNewProjectStore((state) => state.defaults);
  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && close()}>
      <DialogContent className="w-2xl max-w-full">
        <DialogHeader>
          <DialogTitle>New project</DialogTitle>
        </DialogHeader>
        {isOpen && <NewProjectForm defaults={defaults} onClose={close} />}
      </DialogContent>
    </Dialog>
  );
}

function useInitialTeams(defaults: TNewProjectDefaults): TeamRef[] {
  const { teams, myTeams } = useTeams();
  const routeTeam = useTeamFromRoute();
  const id = [defaults.teamId, routeTeam?.id, myTeams[0]?.id, teams[0]?.id].find(Boolean);
  const team = teams.find((candidate) => candidate.id === id);
  return team ? [{ id: team.id, key: team.key, name: team.name }] : [];
}

type TPlannedDate = { date: string | null; resolution: TResolution };

const noDate: TPlannedDate = { date: null, resolution: "day" };

function NewProjectForm({
  defaults,
  onClose,
}: {
  defaults: TNewProjectDefaults;
  onClose: () => void;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { teams: allTeams, isPending: isTeamsPending } = useTeams();
  const { statuses } = useProjectStatuses();
  const { users } = useWorkspaceUserList();
  const mentionUsers = useMentionUsers();
  const uploads = useEditorUploads(workspace.id);
  const labelsById = useProjectLabelsById();
  const initialTeams = useInitialTeams(defaults);
  const [pickedTeamIds, setTeamIds] = useState<string[] | null>(null);
  const teams = pickedTeamIds
    ? allTeams
        .filter((team) => pickedTeamIds.includes(team.id))
        .map((team) => ({ id: team.id, key: team.key, name: team.name }))
    : initialTeams;
  const hasNoTeams = !isTeamsPending && allTeams.length === 0;
  const [statusId, setStatusId] = useState<string | undefined>(defaults.statusId);
  const [leadId, setLeadId] = useState<string | null>(null);
  const [memberIds, setMemberIds] = useState<string[]>([]);
  const [labelIds, setLabelIds] = useState<string[]>([]);
  const [priority, setPriority] = useState(0);
  const [start, setStart] = useState<TPlannedDate>(noDate);
  const [target, setTarget] = useState<TPlannedDate>(noDate);
  const [description, setDescription] = useState("");
  const { mutateAsync: saveProject } = useSaveProjectMutation(workspace.id);
  const status =
    statuses.find((candidate) => candidate.id === statusId) ??
    statuses.find((candidate) => candidate.type === "backlog" && !candidate.archivedAt);
  const lead = users.find((user) => user.id === leadId);
  const members = users.filter((user) => memberIds.includes(user.id)).map(toUserRef);

  const form = useAppForm({
    defaultValues: { name: "", summary: "" },
    validators: { onSubmit: ProjectFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = ProjectFormSchema.parse(value);
      const project = await saveProject({
        name: parsed.name,
        summary: parsed.summary || undefined,
        description: description.trim() || undefined,
        setTeams: teams.map((team) => team.id),
        state: status?.id,
        lead: leadId ?? undefined,
        addMembers: memberIds.length > 0 ? memberIds : undefined,
        labels: labelIds.length > 0 ? labelIds : undefined,
        priority,
        startDate: start.date ?? undefined,
        startDateResolution: start.date ? fromResolution(start.resolution) : undefined,
        targetDate: target.date ?? undefined,
        targetDateResolution: target.date ? fromResolution(target.resolution) : undefined,
        addInitiatives: defaults.initiativeId ? [defaults.initiativeId] : undefined,
      });
      onClose();
      await navigate({
        to: "/$workspace/project/$slug",
        params: { workspace: workspace.urlKey, slug: entitySlug(project) },
      });
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      {hasNoTeams && <NoTeamsNotice entity="Projects" onNavigate={onClose} />}
      <form.AppField name="name" children={(field) => <field.TextField label="Name" autoFocus />} />
      <form.AppField
        name="summary"
        children={(field) => (
          <field.TextField label="Summary" placeholder="One line about the project" />
        )}
      />
      <MarkdownField
        value={description}
        onChange={setDescription}
        users={mentionUsers}
        {...uploads.editorProps}
        placeholder="What is this project about? Write a plan, goals, links…"
        label="Description"
      />
      <div className="flex flex-wrap items-center gap-2">
        <ProjectStatusPicker appearance="compact" status={status} onChange={setStatusId} />
        <TeamsPicker appearance="compact" teams={teams} onChange={setTeamIds} />
        <PersonPicker
          role="Lead"
          appearance="compact"
          user={lead ? toUserRef(lead) : null}
          onChange={setLeadId}
        />
        <PeoplePicker appearance="compact" people={members} onChange={setMemberIds} />
        <ProjectPriorityPicker appearance="compact" priority={priority} onChange={setPriority} />
        <PlannedDatePicker
          appearance="compact"
          label="Start date"
          edge="start"
          date={start.date}
          resolution={start.resolution}
          onChange={setStart}
        />
        <PlannedDatePicker
          appearance="compact"
          label="Target date"
          edge="end"
          date={target.date}
          resolution={target.resolution}
          onChange={setTarget}
        />
        <ProjectLabelsPicker
          appearance="compact"
          labels={labelIds.flatMap((id) => labelsById.get(id) ?? [])}
          onChange={setLabelIds}
        />
      </div>
      <DialogFooter className="items-center">
        {!hasNoTeams && teams.length === 0 && (
          <p className="text-muted-foreground mr-auto text-sm">Pick at least one team.</p>
        )}
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm" disabled={teams.length === 0}>
            Create project
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}
