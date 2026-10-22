import { Link } from "@tanstack/react-router";
import { LinkIcon, PencilIcon, Trash2Icon } from "lucide-react";

import type { CustomerNeed } from "@/api/ops.gen";
import { CustomerLogo, ImportantIcon } from "@/components/customers/customer-badges";
import { CustomerNeedDialog } from "@/components/customers/customer-need-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { ProjectIcon } from "@/components/shared/project-icon";
import { RelativeTime } from "@/components/shared/relative-time";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TNeedScope, useDeleteCustomerNeedMutation } from "@/lib/queries/customers";

type TProps = {
  needs: CustomerNeed[];
  scope: TNeedScope;
  /** What each request names: the issue or project it is for, or the customer who asked. */
  show: "target" | "customer";
};

const linkClassName =
  "flex min-w-0 items-center gap-1.5 font-medium outline-hidden focus-visible:underline has-hover:hover:underline";

export function CustomerNeedList({ needs, scope, show }: TProps) {
  return (
    <ul className="bg-card flex w-full flex-col overflow-hidden rounded-lg border">
      {needs.map((need) => (
        <NeedItem key={need.id} need={need} scope={scope} show={show} />
      ))}
    </ul>
  );
}

function NeedItem({ need, scope, show }: { need: CustomerNeed } & Omit<TProps, "needs">) {
  const { workspace, canEdit } = useWorkspace();
  const { mutateAsync: remove } = useDeleteCustomerNeedMutation(workspace.id);
  return (
    <li className="flex flex-col gap-1 border-b px-3 py-2 last:border-b-0">
      <div className="flex min-h-7 items-center gap-2 text-sm">
        {need.priority === 1 && <ImportantIcon />}
        <div className="flex min-w-0 flex-1 items-center">
          {show === "customer" ? <CustomerLink need={need} /> : <TargetLink need={need} />}
        </div>
        <span className="text-muted-foreground hidden shrink-0 text-xs sm:inline">
          {need.creator ? `${need.creator.name} · ` : ""}
          <RelativeTime value={need.createdAt} />
        </span>
        {canEdit && (
          <span className="flex shrink-0 items-center">
            <CustomerNeedDialog
              scope={scope}
              need={need}
              trigger={
                <Button variant="ghost" size="icon" aria-label="Edit request">
                  <PencilIcon className="size-4" />
                </Button>
              }
            />
            <ConfirmDialog
              trigger={
                <Button variant="ghost" size="icon" aria-label="Delete request">
                  <Trash2Icon className="size-4" />
                </Button>
              }
              title="Delete this request"
              description="The customer and the issue or project stay; only this request goes."
              confirmLabel="Delete request"
              onConfirm={() => remove(need.id)}
            />
          </span>
        )}
      </div>
      {need.body && (
        <p className="text-muted-foreground line-clamp-3 text-sm whitespace-pre-wrap">
          {need.body}
        </p>
      )}
      {need.attachmentUrl && (
        <a
          href={need.attachmentUrl}
          target="_blank"
          rel="noreferrer noopener"
          className="text-muted-foreground has-hover:hover:text-foreground flex w-fit max-w-full items-center gap-1.5 text-xs outline-hidden focus-visible:underline"
        >
          <LinkIcon className="size-3.5 shrink-0" />
          <span className="truncate">{hostOf(need.attachmentUrl)}</span>
        </a>
      )}
    </li>
  );
}

function hostOf(url: string) {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function TargetLink({ need }: { need: CustomerNeed }) {
  const { workspace } = useWorkspace();
  if (need.issue) {
    return (
      <Link
        to="/$workspace/issue/$identifier"
        params={{ workspace: workspace.urlKey, identifier: need.issue.identifier }}
        className={linkClassName}
      >
        <StateIcon type={need.issue.state.type} color={need.issue.state.color} />
        <span className="text-muted-foreground shrink-0 font-mono text-xs font-normal">
          {need.issue.identifier}
        </span>
        <span className="truncate">{need.issue.title}</span>
      </Link>
    );
  }
  if (need.project) {
    return (
      <Link
        to="/$workspace/project/$slug"
        params={{ workspace: workspace.urlKey, slug: need.project.slugId }}
        className={linkClassName}
      >
        <ProjectIcon />
        <span className="truncate">{need.project.name}</span>
      </Link>
    );
  }
  return <span className="text-muted-foreground">Not linked</span>;
}

function CustomerLink({ need }: { need: CustomerNeed }) {
  const { workspace } = useWorkspace();
  if (!need.customer) return <span className="text-muted-foreground">Unknown customer</span>;
  return (
    <Link
      to="/$workspace/customer/$slug"
      params={{ workspace: workspace.urlKey, slug: need.customer.slugId }}
      className={linkClassName}
    >
      <CustomerLogo id={need.customer.id} name={need.customer.name} size="sm" />
      <span className="truncate">{need.customer.name}</span>
    </Link>
  );
}
