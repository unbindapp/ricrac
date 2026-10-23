import { useQuery } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { GitHubRepoSync } from "@/api/ops.gen";
import { LockIcon } from "@/components/icons";
import { GithubIcon } from "@/components/icons/github";
import { ErrorCard } from "@/components/layout/error-card";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, type TSelectFieldItem, useAppForm } from "@/lib/hooks/use-app-form";
import { projectOptionsQuery } from "@/lib/queries/entity-options";
import { githubRepositoriesQuery, useSaveGithubRepoSyncMutation } from "@/lib/queries/github";

const NO_PROJECT = "none";

const formSchema = z.object({
  repository: z.string().min(1, "Pick a repository."),
  project: z.string(),
  includeClosed: z.boolean(),
  backfill: z.boolean(),
});

type TProps = { trigger: ReactElement; repoSync?: GitHubRepoSync };

/** Adds a repository sync, or changes one when repoSync is given. */
export function GithubRepoSyncDialog({ trigger, repoSync }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{repoSync ? repoSync.repository.fullName : "Sync a repository"}</DialogTitle>
          <DialogDescription>
            Issues opened on GitHub become issues here, and changes made in Ricrac go back to
            GitHub.
          </DialogDescription>
        </DialogHeader>
        {isOpen && <RepoSyncForm repoSync={repoSync} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function RepoSyncForm({ repoSync, onDone }: { repoSync?: GitHubRepoSync; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const team = useWorkspaceTeam();
  const repositories = useQuery({ ...githubRepositoriesQuery(workspace.id), enabled: !repoSync });
  const { data: projects = [] } = useQuery(projectOptionsQuery(workspace.id));
  const { mutateAsync: save } = useSaveGithubRepoSyncMutation(workspace.id);

  const form = useAppForm({
    defaultValues: {
      repository: repoSync?.repository.fullName ?? "",
      project: repoSync?.project?.id ?? NO_PROJECT,
      includeClosed: repoSync?.backfill?.includeClosed ?? false,
      backfill: !repoSync,
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const project = value.project === NO_PROJECT ? null : value.project;
      const fields = { project, includeClosed: value.includeClosed };
      if (repoSync) {
        await save({ id: repoSync.id, ...fields, backfill: value.backfill || undefined });
        onDone();
        return;
      }
      await save({
        repository: value.repository,
        team: team?.id,
        ...fields,
        backfill: value.backfill,
      });
      onDone();
    },
  });

  const repositoryItems: TSelectFieldItem[] = (repositories.data ?? [])
    .filter((repository) => !repository.repoSync && repository.hasIssues)
    .map((repository) => ({
      value: repository.fullName,
      label: repository.fullName,
      icon: repository.private ? (
        <LockIcon className="text-muted-foreground size-4" />
      ) : (
        <GithubIcon className="text-muted-foreground" />
      ),
    }));
  const projectItems: TSelectFieldItem[] = [
    { value: NO_PROJECT, label: "No project" },
    ...projects.map((project) => ({
      value: project.id,
      label: project.name,
      icon: <ProjectIcon icon={project.icon} color={project.color} />,
    })),
  ];

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      {repoSync?.lastError && (
        <ErrorCard title="The last change didn't go through" message={repoSync.lastError} />
      )}
      {!repoSync && (
        <RepositoryField
          isPending={repositories.isPending}
          error={repositories.error}
          onRetry={() => void repositories.refetch()}
          isEmpty={repositoryItems.length === 0}
        >
          <form.AppField
            name="repository"
            children={(field) => (
              <field.SelectField
                label="Repository"
                placeholder="Pick a repository"
                items={repositoryItems}
              />
            )}
          />
        </RepositoryField>
      )}
      <form.AppField
        name="project"
        children={(field) => (
          <field.SelectField
            label="Project"
            description="Optional: issues from GitHub also join this project."
            items={projectItems}
          />
        )}
      />
      <form.AppField
        name="backfill"
        children={(field) => (
          <field.SwitchField
            label={repoSync ? "Import existing issues again" : "Import existing issues"}
            description="Copies the repository's issues and their comments in the background. Issues already linked are brought up to date."
          />
        )}
      />
      <form.AppField
        name="includeClosed"
        children={(field) => (
          <field.SwitchField
            label="Include closed issues"
            description="Imports bring closed issues too, not only open ones."
          />
        )}
      />
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm" disabled={!repoSync && repositoryItems.length === 0}>
            {repoSync ? "Save" : "Sync repository"}
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

function RepositoryField({
  isPending,
  error,
  onRetry,
  isEmpty,
  children,
}: {
  isPending: boolean;
  error: Error | null;
  onRetry: () => void;
  isEmpty: boolean;
  children: ReactElement;
}) {
  if (isPending) {
    return (
      <div className="flex flex-col gap-1.5" aria-busy aria-label="Loading repositories">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-9 w-full" />
      </div>
    );
  }
  if (error) {
    return (
      <ErrorCard
        title="Could not list the repositories"
        message={error.message}
        onRetry={onRetry}
      />
    );
  }
  if (isEmpty) {
    return (
      <p className="bg-muted text-muted-foreground dank:rounded-md rounded-lg p-4 text-sm">
        Every repository the app can reach already syncs, or has issues turned off. Give the app
        more repositories on GitHub with Manage.
      </p>
    );
  }
  return children;
}
