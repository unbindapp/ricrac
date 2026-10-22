import { useQuery } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import { DocumentParentPicker } from "@/components/documents/document-parent-picker";
import {
  type TDocumentParent,
  useCreateDocument,
} from "@/components/documents/use-create-document";
import { ChevronDownIcon } from "@/components/icons/symbols.gen";
import { InitiativeIcon } from "@/components/initiatives/initiative-status";
import { ProjectIcon } from "@/components/shared/project-icon";
import { TeamIcon } from "@/components/shared/team-icon";
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
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { DOCUMENT_TITLE_MAX_LENGTH, UNTITLED_DOCUMENT } from "@/lib/queries/documents";
import { initiativesQuery } from "@/lib/queries/initiatives";
import { projectsQuery } from "@/lib/queries/projects";

const schema = z.object({
  title: z.string().trim().max(DOCUMENT_TITLE_MAX_LENGTH, "Keep the title shorter."),
});

export function NewDocumentDialog({ trigger }: { trigger: ReactElement }) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>New document</DialogTitle>
        </DialogHeader>
        {isOpen && <NewDocumentForm />}
      </DialogContent>
    </Dialog>
  );
}

function NewDocumentForm() {
  const { myTeams, teams } = useTeams();
  const firstTeam = myTeams[0] ?? teams[0];
  const [parent, setParent] = useState<TDocumentParent | null>(
    firstTeam ? { team: firstTeam.id } : null,
  );
  const { create } = useCreateDocument();
  const form = useAppForm({
    defaultValues: { title: "" },
    validators: { onSubmit: schema },
    onSubmit: ({ value }) => {
      if (!parent) return;
      create(parent, schema.parse(value).title || UNTITLED_DOCUMENT);
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="title"
        children={(field) => (
          <field.TextField label="Title" placeholder={UNTITLED_DOCUMENT} autoFocus />
        )}
      />
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium">In</span>
        <DocumentParentPicker
          label="Put the document in"
          value={parent}
          onChange={setParent}
          trigger={
            <Button variant="outline" className="justify-start font-normal">
              <ParentLabel parent={parent} />
              <ChevronDownIcon className="text-muted-foreground ml-auto size-4" />
            </Button>
          }
        />
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
          <form.SubmitButton size="sm" disabled={!parent}>
            Create document
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

function ParentLabel({ parent }: { parent: TDocumentParent | null }) {
  const { workspace } = useWorkspace();
  const { teams } = useTeams();
  const projects = useQuery(projectsQuery(workspace.id));
  const initiatives = useQuery(initiativesQuery(workspace.id));
  if (!parent)
    return <span className="text-muted-foreground">Pick a team, project or initiative</span>;
  if ("team" in parent) {
    const team = teams.find((item) => item.id === parent.team);
    if (!team) return null;
    return (
      <>
        <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />
        <span className="truncate">{team.name}</span>
      </>
    );
  }
  if ("project" in parent) {
    const project = projects.data?.find((item) => item.id === parent.project);
    return (
      <>
        <ProjectIcon icon={project?.icon} color={project?.color} />
        <span className="truncate">{project?.name}</span>
      </>
    );
  }
  if ("initiative" in parent) {
    const initiative = initiatives.data?.find((item) => item.id === parent.initiative);
    return (
      <>
        <InitiativeIcon initiative={initiative ?? {}} />
        <span className="truncate">{initiative?.name}</span>
      </>
    );
  }
  return null;
}
