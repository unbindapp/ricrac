import { type LinkProps } from "@tanstack/react-router";
import { type ReactNode } from "react";

import type { Document } from "@/api/ops.gen";
import {
  CircleDotIcon,
  RefreshCwIcon,
  RocketIcon,
  TargetIcon,
} from "@/components/icons/symbols.gen";
import { ProjectIcon } from "@/components/shared/project-icon";
import { TeamIcon } from "@/components/shared/team-icon";
import { useTeams } from "@/components/workspace/teams-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

type TParentFields = Pick<
  Document,
  "project" | "initiative" | "team" | "issue" | "cycle" | "release"
>;

export type TDocumentParentInfo = { label: string; icon: ReactNode; link?: LinkProps };

/** What a document belongs to, with an icon and where it lives in the app. */
export function useDocumentParent(
  document: TParentFields,
  workspaceUrlKey: string,
): TDocumentParentInfo | null {
  const { teamByKey } = useTeams();
  const workspace = workspaceUrlKey;
  if (document.project) {
    return {
      label: document.project.name,
      icon: <ProjectIcon size="sm" />,
      link: {
        to: "/$workspace/project/$slug",
        params: { workspace, slug: entitySlug(document.project) },
      },
    };
  }
  if (document.initiative) {
    return {
      label: document.initiative.name,
      icon: <TargetIcon className="text-muted-foreground size-3.5" />,
      link: {
        to: "/$workspace/initiative/$slug",
        params: { workspace, slug: entitySlug(document.initiative) },
      },
    };
  }
  if (document.issue) {
    return {
      label: `${document.issue.identifier} ${document.issue.title}`,
      icon: <CircleDotIcon className="text-muted-foreground size-3.5" />,
      link: {
        to: "/$workspace/issue/$identifier",
        params: { workspace, identifier: document.issue.identifier },
      },
    };
  }
  if (document.cycle) {
    const { cycle, team } = document;
    return {
      label: cycle.name ?? `Cycle ${cycle.number}`,
      icon: <RefreshCwIcon className="text-muted-foreground size-3.5" />,
      link: team
        ? {
            to: "/$workspace/team/$key/cycle/$number",
            params: { workspace, key: team.key, number: String(cycle.number) },
          }
        : undefined,
    };
  }
  if (document.team) {
    const team = teamByKey(document.team.key);
    return {
      label: document.team.name,
      icon: (
        <TeamIcon teamKey={document.team.key} icon={team?.icon} color={team?.color} size="sm" />
      ),
      link: {
        to: "/$workspace/team/$key/issues",
        params: { workspace, key: document.team.key },
      },
    };
  }
  if (document.release) {
    return {
      label: document.release.name,
      icon: <RocketIcon className="text-muted-foreground size-3.5" />,
    };
  }
  return null;
}
