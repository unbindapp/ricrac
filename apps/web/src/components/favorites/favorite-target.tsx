import { type LinkProps } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { BoxIcon, FileTextIcon, LayersIcon, ListTodoIcon, TargetIcon } from "lucide-react";

import type {
  DocumentRef,
  FavoriteCustomView,
  FavoriteCycle,
  InitiativeRef,
  IssueRef,
  LabelRef,
  ProjectRef,
} from "@/api/ops.gen";
import { cycleStatus } from "@/components/cycles/cycle-status";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { cycleName } from "@/lib/queries/cycles";
import type { TFavorite } from "@/lib/queries/favorites";

type TFavoriteTarget = { label: string; icon: ReactNode; link: LinkProps };

/** What a favorite shows in the sidebar; null for things without a page here (people, releases). */
export function favoriteTarget(workspace: string, favorite: TFavorite): TFavoriteTarget | null {
  switch (favorite.type) {
    case "issue": {
      const issue = favorite.target as IssueRef;
      return {
        label: `${issue.identifier} ${issue.title}`,
        icon: <StateIcon type={issue.state.type} color={issue.state.color} />,
        link: {
          to: "/$workspace/issue/$identifier",
          params: { workspace, identifier: issue.identifier },
        },
      };
    }
    case "project": {
      const project = favorite.target as ProjectRef;
      return {
        label: project.name,
        icon: <BoxIcon className="size-4" />,
        link: { to: "/$workspace/project/$slug", params: { workspace, slug: project.slugId } },
      };
    }
    case "initiative": {
      const initiative = favorite.target as InitiativeRef;
      return {
        label: initiative.name,
        icon: <TargetIcon className="size-4" />,
        link: {
          to: "/$workspace/initiative/$slug",
          params: { workspace, slug: initiative.slugId },
        },
      };
    }
    case "cycle": {
      const cycle = favorite.target as FavoriteCycle;
      return {
        label: cycleName(cycle),
        icon: <CycleIcon status={cycleStatus(cycle)} className="size-4" />,
        link: {
          to: "/$workspace/cycle/$number",
          params: { workspace, number: String(cycle.number) },
        },
      };
    }
    case "customView": {
      const view = favorite.target as FavoriteCustomView;
      return {
        label: view.name,
        icon: <LayersIcon className="size-4" />,
        link: { to: "/$workspace/view/$view_id", params: { workspace, view_id: view.slugId } },
      };
    }
    case "document": {
      const document = favorite.target as DocumentRef;
      return {
        label: document.title,
        icon: <FileTextIcon className="size-4" />,
        link: { to: "/$workspace/document/$slug", params: { workspace, slug: document.slugId } },
      };
    }
    case "label": {
      const label = favorite.target as LabelRef;
      return {
        label: label.parent ? `${label.parent}/${label.name}` : label.name,
        icon: (
          <span className="flex size-4 items-center justify-center">
            <span className="size-2.5 rounded-full" style={{ backgroundColor: label.color }} />
          </span>
        ),
        link: {
          to: "/$workspace/issues",
          params: { workspace },
          search: { label_ids: [label.id] },
        },
      };
    }
    case "team":
      return {
        label: "Issues",
        icon: <ListTodoIcon className="size-4" />,
        link: { to: "/$workspace/issues", params: { workspace } },
      };
  }
  return null;
}
