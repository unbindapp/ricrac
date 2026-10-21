import type { LinkProps } from "@tanstack/react-router";
import {
  BoxIcon,
  FileTextIcon,
  MessageSquareIcon,
  MessagesSquareIcon,
  TargetIcon,
} from "lucide-react";
import { Fragment } from "react";

import { StateIcon } from "@/components/shared/state-icon";
import type { TSearchHit } from "@/lib/queries/search";

export const searchHitHeadings: Record<TSearchHit["type"], string> = {
  issue: "Issues",
  project: "Projects",
  initiative: "Initiatives",
  document: "Documents",
  discussion: "Discussions",
  comment: "Comments",
};

/** The page a hit opens; projects, initiatives, documents and discussions by the slug of their link. */
export function searchHitLink(hit: TSearchHit, workspace: string): LinkProps {
  const slug = new URL(hit.url, window.location.origin).pathname.split("/").pop() ?? hit.id;
  switch (hit.type) {
    case "issue":
      return {
        to: "/$workspace/issue/$identifier",
        params: { workspace, identifier: hit.identifier },
      };
    case "comment":
      return {
        to: "/$workspace/issue/$identifier",
        params: { workspace, identifier: hit.identifier },
        hash: `comment-${hit.id}`,
      };
    case "project":
      return { to: "/$workspace/project/$slug", params: { workspace, slug } };
    case "initiative":
      return { to: "/$workspace/initiative/$slug", params: { workspace, slug } };
    case "document":
      return { to: "/$workspace/document/$slug", params: { workspace, slug } };
    case "discussion":
      return { to: "/$workspace/discussion/$slug", params: { workspace, slug } };
  }
}

export function SearchHitIcon({ hit }: { hit: TSearchHit }) {
  switch (hit.type) {
    case "issue":
      return <StateIcon type={hit.state.type} color={hit.state.color} name={hit.state.name} />;
    case "project":
      return <BoxIcon className="text-muted-foreground size-4" />;
    case "initiative":
      return <TargetIcon className="text-muted-foreground size-4" />;
    case "document":
      return <FileTextIcon className="text-muted-foreground size-4" />;
    case "discussion":
      return <MessagesSquareIcon className="text-muted-foreground size-4" />;
    case "comment":
      return <MessageSquareIcon className="text-muted-foreground size-4" />;
  }
}

/** Where a hit sits: its team, project or status. */
export function searchHitContext(hit: TSearchHit) {
  switch (hit.type) {
    case "issue":
      return hit.team.name;
    case "project":
      return hit.status.name;
    case "document":
      return hit.project?.name;
    case "comment":
      return hit.user
        ? `${hit.user.name || hit.user.displayName} on ${hit.identifier}`
        : hit.identifier;
    case "initiative":
    case "discussion":
      return undefined;
  }
}

/** A snippet with its matches, which the server wraps in **, in bold. */
export function Snippet({ text }: { text: string }) {
  return text.split("**").map((part, index) =>
    index % 2 === 1 ? (
      <strong key={index} className="text-foreground font-medium">
        {part}
      </strong>
    ) : (
      <Fragment key={index}>{part}</Fragment>
    ),
  );
}
