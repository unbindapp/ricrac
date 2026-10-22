import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { type ReactNode, useState } from "react";

import type { TemplateDetail } from "@/api/ops.gen";
import { PlusIcon, UserIcon, XIcon } from "@/components/icons/symbols.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { Avatar } from "@/components/shared/avatar";
import { PriorityIcon, priorityLabel, PRIORITIES } from "@/components/shared/priority-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  dataFromDraft,
  draftFromData,
  type TTemplateDraft,
  type TTemplateKind,
} from "@/components/templates/template-data";
import { Field, MultiPicker, SimpleSelect } from "@/components/templates/template-fields";
import type { TFilterOption } from "@/components/views/use-filter-options";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { labelsQuery } from "@/lib/queries/labels";
import { teamStatusesQuery } from "@/lib/queries/teams";
import { templateQuery, type TTemplate, useSaveTemplateMutation } from "@/lib/queries/templates";
import { usersQuery } from "@/lib/queries/users";

const kindNames: Record<TTemplateKind, string> = {
  issue: "issue",
  project: "project",
  document: "document",
};

export type TTemplateDialogTarget =
  { mode: "create"; kind: TTemplateKind } | { mode: "edit"; template: TTemplate };

type TProps = {
  target: TTemplateDialogTarget | null;
  onClose: () => void;
};

export function TemplateDialog({ target, onClose }: TProps) {
  const kind = target?.mode === "edit" ? target.template.type : target?.kind;
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-2xl max-w-full">
        <DialogHeader>
          <DialogTitle>
            {target?.mode === "edit"
              ? `Edit ${target.template.name}`
              : `New ${kind ? kindNames[kind] : ""} template`}
          </DialogTitle>
          <DialogDescription>
            {kind === "issue" &&
              "New issues start from it; it can also create an issue on a schedule."}
            {kind === "project" && "New projects start with its milestones and issues."}
            {kind === "document" && "New documents start from its title and content."}
          </DialogDescription>
        </DialogHeader>
        {target?.mode === "create" && (
          <TemplateForm kind={target.kind} template={null} onDone={onClose} />
        )}
        {target?.mode === "edit" && <EditTemplate template={target.template} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function EditTemplate({ template, onDone }: { template: TTemplate; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { data, error, refetch } = useQuery(templateQuery(workspace.id, template.id));
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (!data) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  return <TemplateForm kind={data.type} template={data} onDone={onDone} />;
}

/** Issue templates belong to the workspace's team so they can recur; others keep theirs. */
function templateTeam(kind: TTemplateKind, template: TemplateDetail | null, teamId?: string) {
  if (kind === "issue" && teamId) return { team: teamId };
  if (template) return {};
  return { team: null };
}

function localDateTime(value: string | null | undefined) {
  return format(value ? new Date(value) : new Date(), "yyyy-MM-dd'T'HH:mm");
}

function TemplateForm({
  kind,
  template,
  onDone,
}: {
  kind: TTemplateKind;
  template: TemplateDetail | null;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const team = useWorkspaceTeam();
  const { mutate: saveTemplate, isPending } = useSaveTemplateMutation(workspace.id);
  const [name, setName] = useState(template?.name ?? "");
  const [about, setAbout] = useState(template?.description ?? "");
  const [draft, setDraft] = useState<TTemplateDraft>(() =>
    draftFromData(kind, template?.data ?? {}),
  );
  const [repeats, setRepeats] = useState(!!template?.recurrence);
  const [every, setEvery] = useState(String(template?.recurrence?.every ?? 1));
  const [unit, setUnit] = useState<string>(template?.recurrence?.unit ?? "week");
  const [startsAt, setStartsAt] = useState(localDateTime(template?.recurrence?.startsAt));
  const [error, setError] = useState<string | null>(null);
  const edit = (patch: Partial<TTemplateDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const submit = () => {
    if (!name.trim()) return setError("Give the template a name.");
    const everyNumber = Number(every);
    if (repeats && !(Number.isInteger(everyNumber) && everyNumber > 0)) {
      return setError("Repeat every 1 or more.");
    }
    if (repeats && !draft.title.trim()) return setError("Recurring issues need a title.");
    setError(null);
    const recurrence = repeats
      ? { every: everyNumber, unit, startsAt: new Date(startsAt).toISOString() }
      : template?.recurrence
        ? null
        : undefined;
    const fields = {
      name: name.trim(),
      description: about.trim() || null,
      ...templateTeam(kind, template, team?.id),
      data: dataFromDraft(kind, draft, template?.data ?? {}),
      ...(kind === "issue" && recurrence !== undefined ? { recurrence } : {}),
    };
    saveTemplate(template ? { id: template.id, ...fields } : { type: kind, ...fields }, {
      onSuccess: onDone,
    });
  };

  return (
    <form
      className="flex flex-col gap-4"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <div className="flex flex-col gap-6">
        <Field label="Template name">
          {(id) => (
            <Input
              id={id}
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={
                kind === "issue"
                  ? "Bug report"
                  : kind === "project"
                    ? "Feature launch"
                    : "Postmortem"
              }
              autoFocus
              autoComplete="off"
            />
          )}
        </Field>
        <Field label="What it's for">
          {(id) => (
            <Input
              id={id}
              value={about}
              onChange={(event) => setAbout(event.target.value)}
              placeholder="Optional, shown when people pick a template"
              autoComplete="off"
            />
          )}
        </Field>
        <Section
          title={
            kind === "issue" ? "New issues" : kind === "project" ? "New projects" : "New documents"
          }
        >
          {kind === "issue" && <IssueFields draft={draft} edit={edit} teamId={team?.id} />}
          {kind === "project" && <ProjectFields draft={draft} edit={edit} />}
          {kind === "document" && <DocumentFields draft={draft} edit={edit} />}
        </Section>
        {kind === "issue" && (
          <Section
            title="Repeat"
            action={
              <Switch
                checked={repeats}
                onCheckedChange={setRepeats}
                aria-label="Create issues on a schedule"
              />
            }
          >
            {repeats ? (
              <div className="flex flex-col gap-1.5">
                <div className="flex flex-wrap items-center gap-2 text-sm">
                  <span>Create an issue every</span>
                  <Input
                    aria-label="Every"
                    type="number"
                    min={1}
                    value={every}
                    onChange={(event) => setEvery(event.target.value)}
                    className="w-16"
                  />
                  <div className="w-28">
                    <SimpleSelect
                      label="Unit"
                      value={unit}
                      onChange={setUnit}
                      items={[
                        { value: "day", label: Number(every) === 1 ? "day" : "days" },
                        { value: "week", label: Number(every) === 1 ? "week" : "weeks" },
                        { value: "month", label: Number(every) === 1 ? "month" : "months" },
                      ]}
                    />
                  </div>
                  <span>from</span>
                  <Input
                    aria-label="Starting"
                    type="datetime-local"
                    value={startsAt}
                    onChange={(event) => setStartsAt(event.target.value)}
                    className="w-52"
                  />
                </div>
                <p className="text-muted-foreground text-xs">
                  {template?.nextRunAt
                    ? `Next one ${format(new Date(template.nextRunAt), "EEE, MMM d 'at' HH:mm")}. `
                    : ""}
                  Issues are created by whoever made the template.
                </p>
              </div>
            ) : (
              <p className="text-muted-foreground text-sm">
                Turn on to open this issue on a schedule, like a weekly chore.
              </p>
            )}
          </Section>
        )}
      </div>
      {error && (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      )}
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <Button type="submit" size="sm" isPending={isPending}>
          {template ? "Save" : "Create template"}
        </Button>
      </DialogFooter>
    </form>
  );
}

function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-4 border-t pt-4">
      <div className="flex items-center gap-2">
        <h3 className="flex-1 text-sm font-semibold">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

type TFieldsProps = {
  draft: TTemplateDraft;
  edit: (patch: Partial<TTemplateDraft>) => void;
};

const priorityItems: TFilterOption[] = PRIORITIES.map((priority) => ({
  value: String(priority),
  label: priorityLabel(priority),
  icon: <PriorityIcon priority={priority} />,
}));

function usePeople(noneLabel: string, withMe: boolean): TFilterOption[] {
  const { workspace } = useWorkspace();
  const { data: users = [] } = useQuery(usersQuery(workspace.id));
  return [
    { value: "", label: noneLabel },
    ...(withMe
      ? [{ value: "me", label: "Whoever uses it", icon: <UserIcon className="size-4" /> }]
      : []),
    ...users
      .filter((user) => user.active)
      .map((user) => ({
        value: user.id,
        label: user.displayName,
        icon: <Avatar id={user.id} name={user.displayName} size="xs" />,
      })),
  ];
}

function IssueFields({ draft, edit, teamId }: TFieldsProps & { teamId: string | undefined }) {
  const { workspace } = useWorkspace();
  const people = usePeople("Nobody", true);
  const { data: labels = [] } = useQuery(labelsQuery(workspace.id));
  const { data: statuses = [] } = useQuery({
    ...teamStatusesQuery(workspace.id, teamId ?? ""),
    enabled: !!teamId,
  });
  const labelOptions = labels
    .filter((label) => !label.isGroup)
    .map((label) => ({
      value: label.parent ? `${label.parent.name}/${label.name}` : label.name,
      label: label.parent ? `${label.parent.name}/${label.name}` : label.name,
      icon: (
        <span className="flex size-4 items-center justify-center">
          <span className="size-2.5 rounded-full" style={{ backgroundColor: label.color }} />
        </span>
      ),
    }));
  const statusItems: TFilterOption[] = [
    { value: "", label: "Default" },
    ...statuses.map((state) => ({
      value: state.name,
      label: state.name,
      icon: <StateIcon type={state.type} color={state.color} />,
    })),
  ];

  return (
    <>
      <Field label="Title">
        {(id) => (
          <Input
            id={id}
            value={draft.title}
            onChange={(event) => edit({ title: event.target.value })}
            placeholder="Bug: "
            autoComplete="off"
          />
        )}
      </Field>
      <Field label="Description">
        {(id) => (
          <Textarea
            id={id}
            value={draft.description}
            onChange={(event) => edit({ description: event.target.value })}
            rows={5}
            placeholder="Optional, in Markdown"
            className="font-mono text-xs"
          />
        )}
      </Field>
      <div className="grid grid-cols-3 gap-4">
        <Field label="Priority">
          {(id) => (
            <SimpleSelect
              id={id}
              label="Priority"
              items={priorityItems}
              value={draft.priority}
              onChange={(priority) => edit({ priority })}
            />
          )}
        </Field>
        <Field label="Assignee">
          {(id) => (
            <SimpleSelect
              id={id}
              label="Assignee"
              items={people}
              value={draft.assignee}
              onChange={(assignee) => edit({ assignee })}
            />
          )}
        </Field>
        <Field label="Status">
          {(id) => (
            <SimpleSelect
              id={id}
              label="Status"
              items={statusItems}
              value={draft.state}
              onChange={(state) => edit({ state })}
            />
          )}
        </Field>
      </div>
      <Field label="Labels">
        {(id) => (
          <MultiPicker
            id={id}
            label="Labels"
            options={labelOptions}
            values={draft.labels}
            onChange={(values) => edit({ labels: values })}
            placeholder="No labels"
          />
        )}
      </Field>
      <ListEditor
        title="Sub-issues"
        addLabel="Add sub-issue"
        items={draft.subIssues}
        onChange={(subIssues) => edit({ subIssues })}
        blank={{ title: "", priority: "0" }}
        render={(item, update) => (
          <>
            <Input
              aria-label="Sub-issue title"
              value={item.title}
              onChange={(event) => update({ ...item, title: event.target.value })}
              placeholder="Title"
              className="flex-1"
            />
            <div className="w-36 shrink-0">
              <SimpleSelect
                label="Sub-issue priority"
                items={priorityItems}
                value={item.priority}
                onChange={(priority) => update({ ...item, priority })}
              />
            </div>
          </>
        )}
      />
    </>
  );
}

function ProjectFields({ draft, edit }: TFieldsProps) {
  const people = usePeople("No lead", false);
  const milestoneItems: TFilterOption[] = [
    { value: "", label: "No milestone" },
    ...draft.milestones
      .filter((milestone) => milestone.name.trim())
      .map((milestone) => ({ value: milestone.name.trim(), label: milestone.name.trim() })),
  ];

  return (
    <>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Name">
          {(id) => (
            <Input
              id={id}
              value={draft.title}
              onChange={(event) => edit({ title: event.target.value })}
              placeholder="Launch: "
              autoComplete="off"
            />
          )}
        </Field>
        <Field label="Summary">
          {(id) => (
            <Input
              id={id}
              value={draft.summary}
              onChange={(event) => edit({ summary: event.target.value })}
              placeholder="One line"
              autoComplete="off"
            />
          )}
        </Field>
      </div>
      <Field label="Description">
        {(id) => (
          <Textarea
            id={id}
            value={draft.description}
            onChange={(event) => edit({ description: event.target.value })}
            rows={4}
            placeholder="Optional, in Markdown"
            className="font-mono text-xs"
          />
        )}
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Priority">
          {(id) => (
            <SimpleSelect
              id={id}
              label="Priority"
              items={priorityItems}
              value={draft.priority}
              onChange={(priority) => edit({ priority })}
            />
          )}
        </Field>
        <Field label="Lead">
          {(id) => (
            <SimpleSelect
              id={id}
              label="Lead"
              items={people}
              value={draft.lead}
              onChange={(lead) => edit({ lead })}
            />
          )}
        </Field>
      </div>
      <ListEditor
        title="Milestones"
        addLabel="Add milestone"
        items={draft.milestones}
        onChange={(milestones) => edit({ milestones })}
        blank={{ name: "", targetDate: "" }}
        render={(item, update) => (
          <>
            <Input
              aria-label="Milestone name"
              value={item.name}
              onChange={(event) => update({ ...item, name: event.target.value })}
              placeholder="Beta"
              className="flex-1"
            />
            <Input
              aria-label="Target date"
              type="date"
              value={item.targetDate}
              onChange={(event) => update({ ...item, targetDate: event.target.value })}
              className="w-40 shrink-0"
            />
          </>
        )}
      />
      <ListEditor
        title="Issues"
        addLabel="Add issue"
        items={draft.issues}
        onChange={(issues) => edit({ issues })}
        blank={{ title: "", milestone: "" }}
        render={(item, update) => (
          <>
            <Input
              aria-label="Issue title"
              value={item.title}
              onChange={(event) => update({ ...item, title: event.target.value })}
              placeholder="Title"
              className="flex-1"
            />
            <div className="w-40 shrink-0">
              <SimpleSelect
                label="Milestone"
                items={milestoneItems}
                value={item.milestone}
                onChange={(milestone) => update({ ...item, milestone })}
              />
            </div>
          </>
        )}
      />
    </>
  );
}

function DocumentFields({ draft, edit }: TFieldsProps) {
  return (
    <>
      <Field label="Title">
        {(id) => (
          <Input
            id={id}
            value={draft.title}
            onChange={(event) => edit({ title: event.target.value })}
            placeholder="Postmortem: "
            autoComplete="off"
          />
        )}
      </Field>
      <Field label="Content">
        {(id) => (
          <Textarea
            id={id}
            value={draft.description}
            onChange={(event) => edit({ description: event.target.value })}
            rows={10}
            placeholder="Markdown"
            className="font-mono text-xs"
          />
        )}
      </Field>
    </>
  );
}

function ListEditor<T>({
  title,
  addLabel,
  items,
  onChange,
  blank,
  render,
}: {
  title: string;
  addLabel: string;
  items: T[];
  onChange: (items: T[]) => void;
  blank: T;
  render: (item: T, update: (item: T) => void) => ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-sm font-medium">{title}</span>
      {items.map((item, index) => (
        <div key={index} className="flex items-center gap-2">
          {render(item, (next) => onChange(items.map((old, n) => (n === index ? next : old))))}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={`Remove from ${title.toLowerCase()}`}
            onClick={() => onChange(items.filter((_, n) => n !== index))}
            className="text-muted-foreground shrink-0"
          >
            <XIcon className="size-4" />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={() => onChange([...items, blank])}
        className="text-muted-foreground self-start"
      >
        <PlusIcon className="size-4" />
        {addLabel}
      </Button>
    </div>
  );
}
