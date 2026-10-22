import { type LinkProps } from "@tanstack/react-router";
import { type ReactNode } from "react";
import { CircleDotIcon, RefreshCwIcon, RocketIcon, TargetIcon } from "lucide-react";

import type { Document } from "@/api/ops.gen";
import { ProjectIcon } from "@/components/shared/project-icon";
import { entitySlug } from "@/lib/helpers/entity-slug";

type TParentFields = Pick<
  Document,
  "project" | "initiative" | "team" | "issue" | "cycle" | "release"
>;

export type TDocumentParentInfo = { label: string; icon: ReactNode; link?: LinkProps };

/** What a document belongs to, with an icon and where it lives in the app; null for the workspace. */
export function useDocumentParent(
  document: TParentFields,
  workspaceUrlKey: string,
): TDocumentParentInfo | null {
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
    const { cycle } = document;
    return {
      label: cycle.name ?? `Cycle ${cycle.number}`,
      icon: <RefreshCwIcon className="text-muted-foreground size-3.5" />,
      link: {
        to: "/$workspace/cycle/$number",
        params: { workspace, number: String(cycle.number) },
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
