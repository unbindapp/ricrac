import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useEffect, useState } from "react";

import type { IssueDetail as TIssueDetail } from "@/api/ops.gen";
import { CustomerNeedDialog } from "@/components/customers/customer-need-dialog";
import { CustomerNeedsSection } from "@/components/customers/customer-needs-section";
import { GithubIcon } from "@/components/icons/github";
import {
  ArchiveIcon,
  CheckIcon,
  CircleDotIcon,
  HandshakeIcon,
  MoonIcon,
  Trash2Icon,
  TriangleIcon,
  UserPlusIcon,
  XIcon,
} from "@/components/icons/symbols.gen";
import { IssueActivity } from "@/components/issues/issue-activity";
import { AttachMenu, IssueAttachments } from "@/components/issues/issue-attachments";
import { IssueDetailHeader } from "@/components/issues/issue-detail-header";
import { draftFromIssue } from "@/components/issues/issue-draft";
import { IssueProperties } from "@/components/issues/issue-properties";
import {
  AddRelationPicker,
  hasRelations,
  IssueRelations,
} from "@/components/issues/issue-relations";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { DatePicker, PickerTrigger, UserAvatar } from "@/components/issues/pickers";
import { PropertyPicker } from "@/components/issues/property-picker";
import { AddReactionButton, groupReactions, ReactionBar } from "@/components/issues/reaction-bar";
import { AddSubIssueMenu, SubIssues } from "@/components/issues/sub-issues";
import { useTriage } from "@/components/issues/triage-actions";
import { useIssueLifecycle, useIssuePatch } from "@/components/issues/use-issue-actions";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PropertyRow } from "@/components/layout/property-row";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { useNow } from "@/components/providers/now-provider";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { InlineText } from "@/components/shared/inline-text";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDateTime } from "@/lib/helpers/dates";
import {
  ISSUE_TITLE_MAX_LENGTH,
  issueQuery,
  useIssueReactionMutation,
  useIssueSubscriptionMutation,
  useSaveDescriptionMutation,
  useShareIssueMutation,
  useUpdateIssueMutation,
} from "@/lib/queries/issues";
import { customerNeedsQuery } from "@/lib/queries/customers";
import { meQuery } from "@/lib/queries/me";
import { isApiError } from "@/lib/server/errors";

type TProps = {
  identifier: string;
  variant?: "page" | "peek";
  onClose?: () => void;
  shortcuts?: boolean;
};

export function IssueDetail({ identifier, variant = "page", onClose, shortcuts = true }: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(issueQuery(workspace.id, identifier));
  const navigate = useNavigate();
  const canonical = data?.identifier;
  useEffect(() => {
    if (variant !== "page" || !canonical || canonical === identifier) return;
    void navigate({
      to: "/$workspace/issue/$identifier",
      params: { workspace: workspace.urlKey, identifier: canonical },
      replace: true,
    });
  }, [variant, canonical, identifier, navigate, workspace.urlKey]);
  if (isPending) {
    return (
      <div className="flex w-full flex-col gap-6 p-4">
        <ListSkeleton rows={4} />
      </div>
    );
  }
  if (error) {
    const description = `There is no issue ${identifier.toUpperCase()}, or you cannot see it.`;
    if (isApiError(error, 404) && variant === "page") {
      return <EntityNotFound Icon={CircleDotIcon} description={description} />;
    }
    if (isApiError(error, 404)) {
      return <EmptyState icon={CircleDotIcon} title="Issue not found" description={description} />;
    }
    return (
      <div className="p-4">
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </div>
    );
  }
  return (
    <IssueDetailContent issue={data} variant={variant} onClose={onClose} shortcuts={shortcuts} />
  );
}

function IssueDetailContent({
  issue,
  variant,
  onClose,
  shortcuts,
}: {
  issue: TIssueDetail;
  variant: "page" | "peek";
  onClose?: () => void;
  shortcuts: boolean;
}) {
  const { workspace, canEdit } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  const patch = useIssuePatch();
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  const { mutateAsync: saveDescription } = useSaveDescriptionMutation(workspace.id);
  const { mutate: setSubscribed } = useIssueSubscriptionMutation(workspace.id);
  const { mutate: toggleReaction } = useIssueReactionMutation(workspace.id);
  const [isSettingSla, setIsSettingSla] = useState(false);
  const { size } = useDeviceSize();

  const isPeek = variant === "peek";
  const hasPropertiesAside = size === "lg" || size === "xl";
  const { data: customerNeeds } = useQuery({
    ...customerNeedsQuery(workspace.id, { issue: issue.identifier }),
    enabled: canEdit,
  });
  const hasCustomerNeeds = (customerNeeds?.length ?? 0) > 0;
  const reactions = groupReactions(issue.reactions, me?.id);
  const react = (emoji: string, reacted: boolean) =>
    toggleReaction({ issueId: issue.id, emoji, reacted });
  const properties = (
    <IssueProperties
      teamId={issue.team.id}
      issueId={issue.id}
      layout={isPeek ? "inline" : "column"}
      hideEmpty={isPeek}
      value={draftFromIssue(issue)}
      onChange={(next) => patch(issue, next)}
      disabled={!canEdit}
      shortcutsEnabled={shortcuts}
      subscribers={{
        users: issue.subscribers,
        onChange: (added, removed) => {
          if (added.length > 0) {
            setSubscribed({ id: issue.id, users: added.map((u) => u.id), subscribed: true });
          }
          if (removed.length > 0) {
            setSubscribed({ id: issue.id, users: removed.map((u) => u.id), subscribed: false });
          }
        },
      }}
    >
      <GitHubIssueRow issue={issue} />
      <SharedWithRow issue={issue} />
      <SlaRow issue={issue} isOpen={isSettingSla} onOpenChange={setIsSettingSla} />
      <SnoozedRow issue={issue} />
    </IssueProperties>
  );

  const main = (
    <div className="flex min-w-0 flex-1 flex-col gap-8">
      <LifecycleBanner issue={issue} />
      <TriageBanner issue={issue} />
      <div className="flex flex-col gap-2">
        <InlineText
          label="Issue title"
          value={issue.title}
          maxLength={ISSUE_TITLE_MAX_LENGTH}
          readOnly={!canEdit}
          onSave={(title) =>
            updateIssue({ id: issue.id, changes: { title }, optimistic: { title } })
          }
        />
        <AutosaveMarkdown
          key={issue.id}
          label="Description"
          placeholder="Add a description…"
          value={issue.description ?? ""}
          readOnly={!canEdit}
          onSave={async (after, before) => {
            const saved = await saveDescription({ id: issue.id, edit: { before, after } });
            return saved.description ?? "";
          }}
        />
        {reactions.length > 0 && (
          <ReactionBar reactions={reactions} disabled={!canEdit} onToggle={react} />
        )}
      </div>
      {!isPeek && !hasPropertiesAside && (
        <section aria-label="Properties" className="rounded-lg border p-3">
          {properties}
        </section>
      )}
      <SubIssues issue={issue} />
      <IssueRelations issue={issue} />
      <IssueAttachments issue={issue} />
      {hasCustomerNeeds && <CustomerNeedsSection issue={issue.identifier} />}
      <EmptyExtras
        issue={issue}
        customerNeed={
          !hasCustomerNeeds && (
            <CustomerNeedDialog
              scope={{ issue: issue.identifier }}
              trigger={
                <Button variant="ghost" size="sm" className="text-muted-foreground font-normal">
                  <HandshakeIcon className="size-4" />
                  Add customer request
                </Button>
              }
            />
          )
        }
        reaction={
          reactions.length === 0 && <AddReactionButton reactions={reactions} onToggle={react} />
        }
      />
      <IssueActivity issueId={issue.id} />
    </div>
  );

  if (isPeek) {
    return (
      <div className="flex w-full flex-col gap-4 p-4">
        <IssueDetailHeader
          issue={issue}
          variant="peek"
          onClose={onClose}
          onSetSla={() => setIsSettingSla(true)}
        />
        {properties}
        {main}
      </div>
    );
  }

  return (
    <>
      <IssueDetailHeader issue={issue} variant="page" onSetSla={() => setIsSettingSla(true)} />
      <div className="-mx-4 -mt-4 -mb-8 flex flex-1">
        <div className="flex min-w-0 flex-1 justify-center px-4 py-6 sm:px-10 sm:py-8">
          <div className="flex w-full max-w-3xl">{main}</div>
        </div>
        {hasPropertiesAside && (
          <aside
            aria-label="Properties"
            className="flex w-80 shrink-0 flex-col gap-2 border-l px-5 py-4"
          >
            <h2 className="text-muted-foreground flex h-7 items-center text-xs font-medium">
              Properties
            </h2>
            {properties}
          </aside>
        )}
      </div>
    </>
  );
}

/** One small row of "add" actions for the sections an issue doesn't have yet. */
function EmptyExtras({
  issue,
  customerNeed,
  reaction,
}: {
  issue: TIssueDetail;
  customerNeed: ReactNode;
  reaction: ReactNode;
}) {
  const { canEdit } = useWorkspace();
  if (!canEdit) return null;
  const actions: ReactNode[] = [];
  if (issue.children.length === 0) actions.push(<AddSubIssueMenu key="sub" issue={issue} />);
  if (!hasRelations(issue)) actions.push(<AddRelationPicker key="relation" issue={issue} />);
  if (issue.attachments.length === 0) actions.push(<AttachMenu key="attach" issue={issue} />);
  if (actions.length === 0 && !customerNeed && !reaction) return null;
  return (
    <div className="-ml-2.5 flex flex-wrap items-center gap-1">
      {actions}
      {customerNeed}
      {reaction}
    </div>
  );
}

function LifecycleBanner({ issue }: { issue: TIssueDetail }) {
  const { canEdit } = useWorkspace();
  const lifecycle = useIssueLifecycle();
  if (!issue.trashedAt && !issue.archivedAt) return null;
  const isTrashed = Boolean(issue.trashedAt);
  const Icon = isTrashed ? Trash2Icon : ArchiveIcon;
  return (
    <div role="status" className="bg-muted flex items-center gap-2 rounded-lg border p-3 text-sm">
      <Icon className="text-muted-foreground size-4 shrink-0" />
      <p className="min-w-0 flex-1">
        {isTrashed
          ? "This issue is in the trash and is deleted for good 30 days after it was trashed."
          : "This issue is archived."}
      </p>
      {canEdit && (
        <Button
          variant="outline"
          size="sm"
          isPending={lifecycle.isPending}
          onClick={() =>
            void lifecycle
              .run([issue.id], isTrashed ? "restore" : "unarchive")
              .catch(() => undefined)
          }
        >
          {isTrashed ? "Restore" : "Unarchive"}
        </Button>
      )}
    </div>
  );
}

function TriageBanner({ issue }: { issue: TIssueDetail }) {
  const { canEdit } = useWorkspace();
  const { target, needsPriority, accept, decline } = useTriage(issue);
  if (issue.state.type !== "triage" || !canEdit) return null;
  return (
    <div
      role="status"
      className="bg-muted flex flex-wrap items-center gap-2 rounded-lg border p-3 text-sm"
    >
      <TriangleIcon className="text-muted-foreground size-4 shrink-0" />
      <p className="min-w-0 flex-1">
        {needsPriority
          ? "This issue is in triage. Set a priority to accept it."
          : `This issue is in triage.${target ? ` Accepting it moves it to ${target.name}.` : ""}`}
      </p>
      {decline && (
        <Button variant="outline" size="sm" onClick={decline}>
          <XIcon className="size-4" />
          Decline
        </Button>
      )}
      <Button size="sm" disabled={!accept} onClick={accept}>
        <CheckIcon className="size-4" />
        Accept
      </Button>
    </div>
  );
}

/** Guests see only their teams' issues and those shared with them. */
function GitHubIssueRow({ issue }: { issue: TIssueDetail }) {
  if (!issue.github) return null;
  return (
    <PropertyRow label="GitHub">
      <a
        href={issue.github.url}
        target="_blank"
        rel="noreferrer noopener"
        className="flex min-w-0 flex-1 items-center gap-1.5 px-2.5 text-sm outline-hidden focus-visible:underline has-hover:hover:underline"
      >
        <GithubIcon className="text-muted-foreground" />
        <span className="shrink-0">#{issue.github.number}</span>
        <span className="text-muted-foreground truncate text-xs">{issue.github.repository}</span>
      </a>
    </PropertyRow>
  );
}

function SharedWithRow({ issue }: { issue: TIssueDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const { mutate: share } = useShareIssueMutation(workspace.id);
  const guests = users.filter((user) => user.role === "guest");
  if (guests.length === 0 && issue.sharedWith.length === 0) return null;
  const current = issue.sharedWith.map((user) => user.id);
  return (
    <PropertyRow label="Shared with">
      <PropertyPicker
        label="Share with guests"
        multiple
        disabled={!canEdit}
        emptyText="No guests in this workspace"
        options={guests.map((user) => ({
          value: user.id,
          label: user.name || user.displayName,
          description: user.email,
          icon: <UserAvatar user={user} size="xs" />,
        }))}
        value={current}
        onChange={(ids) => {
          ids
            .filter((id) => !current.includes(id))
            .forEach((user) => share({ id: issue.id, user, shared: true }));
          current
            .filter((id) => !ids.includes(id))
            .forEach((user) => share({ id: issue.id, user, shared: false }));
        }}
        trigger={
          <PickerTrigger
            label="Share with guests"
            icon={<UserPlusIcon className="text-muted-foreground size-4" />}
            text={
              issue.sharedWith.length === 0
                ? "Not shared"
                : issue.sharedWith.map((user) => user.name || user.displayName).join(", ")
            }
            muted={issue.sharedWith.length === 0}
          />
        }
      />
    </PropertyRow>
  );
}

/** Shown once the issue has an SLA, or while one is being set from the actions menu. */
function SlaRow({
  issue,
  isOpen,
  onOpenChange,
}: {
  issue: TIssueDetail;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  if (!issue.slaBreachesAt && !isOpen) return null;
  const save = (slaBreachesAt: string | null) =>
    updateIssue({ id: issue.id, changes: { slaBreachesAt }, optimistic: { slaBreachesAt } });
  return (
    <PropertyRow label="SLA">
      <DatePicker
        name="SLA"
        emptyLabel="Pick when it breaches"
        date={issue.slaBreachesAt}
        format={formatDateTime}
        open={isOpen}
        onOpenChange={onOpenChange}
        disabled={!canEdit}
        onChange={(date) =>
          save(date ? new Date(`${date.slice(0, 10)}T23:59:59`).toISOString() : null)
        }
      />
    </PropertyRow>
  );
}

function SnoozedRow({ issue }: { issue: TIssueDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const now = useNow();
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  if (!issue.snoozedUntil || new Date(issue.snoozedUntil).getTime() < now) return null;
  return (
    <PropertyRow label="Snoozed">
      <span className="flex min-w-0 flex-1 items-center gap-1.5 px-2.5 text-sm">
        <MoonIcon className="text-muted-foreground size-4 shrink-0" />
        <span className="truncate">Until {formatDateTime(issue.snoozedUntil)}</span>
      </span>
      {canEdit && (
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground font-normal"
          onClick={() =>
            updateIssue({
              id: issue.id,
              changes: { snoozedUntil: null },
              optimistic: { snoozedUntil: null },
            })
          }
        >
          Wake up
        </Button>
      )}
    </PropertyRow>
  );
}
