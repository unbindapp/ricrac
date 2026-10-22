import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { useState } from "react";

import {
  BoxIcon,
  CircleDotIcon,
  EllipsisIcon,
  FileTextIcon,
  PencilIcon,
  PlusIcon,
  RepeatIcon,
  type TIcon,
  Trash2Icon,
} from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { recurrenceText, type TTemplateKind } from "@/components/templates/template-data";
import { TemplateDialog, type TTemplateDialogTarget } from "@/components/templates/template-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { templatesQuery, type TTemplate, useDeleteTemplateMutation } from "@/lib/queries/templates";

const KINDS: { kind: TTemplateKind; title: string; icon: TIcon; about: string }[] = [
  {
    kind: "issue",
    title: "Issue templates",
    icon: CircleDotIcon,
    about: "Prefill new issues, with sub-issues; recurring ones open an issue on a schedule.",
  },
  {
    kind: "project",
    title: "Project templates",
    icon: BoxIcon,
    about: "Start projects with their milestones and first issues.",
  },
  {
    kind: "document",
    title: "Document templates",
    icon: FileTextIcon,
    about: "Start documents from a shared outline.",
  },
];

type TProps = {
  /** Only this team's templates; null for all of them. */
  team: { id: string; name: string } | null;
  canEdit: boolean;
};

/** A section per kind of template; kinds without templates collapse to a create button. */
export function TemplateManager({ team, canEdit }: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(templatesQuery(workspace.id));
  const [dialog, setDialog] = useState<TTemplateDialogTarget | null>(null);
  if (isPending) return <ListSkeleton rows={4} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  const templates = data.filter((template) => !team || template.team?.id === team.id);
  const create = (kind: TTemplateKind) =>
    setDialog({ mode: "create", kind, team: team?.id ?? null });

  return (
    <>
      {KINDS.map(({ kind, title, icon, about }) => {
        const ofKind = templates.filter((template) => template.type === kind);
        return (
          <SettingsSection
            key={kind}
            title={title}
            description={about}
            actions={
              canEdit && (
                <Button
                  variant={ofKind.length ? "outline" : "ghost"}
                  size="sm"
                  onClick={() => create(kind)}
                >
                  <PlusIcon className="size-4" />
                  New template
                </Button>
              )
            }
          >
            {ofKind.length > 0 && (
              <List>
                {ofKind.map((template) => (
                  <TemplateRow
                    key={template.id}
                    template={template}
                    icon={icon}
                    showTeam={!team}
                    canEdit={canEdit}
                    onEdit={() => setDialog({ mode: "edit", template })}
                  />
                ))}
              </List>
            )}
          </SettingsSection>
        );
      })}
      {templates.length === 0 && !canEdit && (
        <EmptyState icon={FileTextIcon} title="No templates yet" />
      )}
      <TemplateDialog target={dialog} onClose={() => setDialog(null)} />
    </>
  );
}

function TemplateRow({
  template,
  icon: Icon,
  showTeam,
  canEdit,
  onEdit,
}: {
  template: TTemplate;
  icon: TIcon;
  showTeam: boolean;
  canEdit: boolean;
  onEdit: () => void;
}) {
  const { workspace } = useWorkspace();
  const [isDeleting, setIsDeleting] = useState(false);
  const { mutateAsync: deleteTemplate } = useDeleteTemplateMutation(workspace.id);
  return (
    <div
      role="listitem"
      className="flex h-9 w-full items-center gap-2 border-b px-3 text-sm last:border-b-0"
    >
      <Icon className="text-muted-foreground size-4 shrink-0" />
      <span className="shrink-0 truncate font-medium">{template.name}</span>
      {template.description && (
        <span className="text-muted-foreground min-w-0 truncate">{template.description}</span>
      )}
      <span className="flex-1" />
      {template.recurrence && (
        <SimpleTooltip
          content={
            template.nextRunAt
              ? `Next ${format(new Date(template.nextRunAt), "EEE, MMM d 'at' HH:mm")}`
              : "Recurring"
          }
        >
          <Badge variant="process">
            <RepeatIcon />
            {recurrenceText(template.recurrence)}
          </Badge>
        </SimpleTooltip>
      )}
      {showTeam && template.team && <Badge variant="outline">{template.team.key}</Badge>}
      {canEdit && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label={`Actions for ${template.name}`}
                className="text-muted-foreground -mr-1 size-7"
              >
                <EllipsisIcon className="size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuItem onClick={onEdit}>
              <PencilIcon className="size-4" />
              Edit template
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
              <Trash2Icon className="size-4" />
              Delete template
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title={`Delete ${template.name}`}
        description={
          template.recurrence
            ? "It stops creating issues. Issues made from it stay as they are."
            : "Things made from it stay as they are."
        }
        confirmLabel="Delete template"
        onConfirm={() => deleteTemplate(template.id)}
      />
    </div>
  );
}
