import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { useQuery } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { CustomerStatus } from "@/api/ops.gen";
import { OptionDot } from "@/components/customers/customer-badges";
import {
  CircleDotIcon,
  GripVerticalIcon,
  LayersIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
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
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { PRESET_COLORS } from "@/lib/helpers/colors";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import {
  customerStatusesQuery,
  customerTiersQuery,
  useDeleteCustomerStatusMutation,
  useDeleteCustomerTierMutation,
  useSaveCustomerStatusMutation,
  useSaveCustomerTierMutation,
} from "@/lib/queries/customers";

/** Statuses and tiers have the same shape and the same settings. */
type TOption = CustomerStatus;
type TKind = "status" | "tier";

const copy = {
  status: {
    title: "Statuses",
    description: "Where each customer stands, like Active or Churned. Drag to reorder.",
    noun: "status",
    empty: "No statuses yet",
    icon: CircleDotIcon,
  },
  tier: {
    title: "Tiers",
    description:
      "How you group customers by plan or size, like Enterprise or Growth. Drag to reorder.",
    noun: "tier",
    empty: "No tiers yet",
    icon: LayersIcon,
  },
} as const;

function useOptions(kind: TKind) {
  const { workspace } = useWorkspace();
  const statuses = useQuery({ ...customerStatusesQuery(workspace.id), enabled: kind === "status" });
  const tiers = useQuery({ ...customerTiersQuery(workspace.id), enabled: kind === "tier" });
  const saveStatus = useSaveCustomerStatusMutation(workspace.id);
  const saveTier = useSaveCustomerTierMutation(workspace.id);
  const deleteStatus = useDeleteCustomerStatusMutation(workspace.id);
  const deleteTier = useDeleteCustomerTierMutation(workspace.id);
  return kind === "status"
    ? { query: statuses, save: saveStatus.mutateAsync, remove: deleteStatus.mutateAsync }
    : { query: tiers, save: saveTier.mutateAsync, remove: deleteTier.mutateAsync };
}

export function CustomerOptionSection({ kind }: { kind: TKind }) {
  const text = copy[kind];
  const { query, save } = useOptions(kind);
  const options = query.data ?? [];
  const [dragOrder, setDragOrder] = useState<string[] | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const ordered = dragOrder
    ? dragOrder
        .map((id) => options.find((option) => option.id === id))
        .filter((option): option is TOption => !!option)
    : options;

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const ids = ordered.map((option) => option.id);
    const next = arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)));
    const index = next.indexOf(String(active.id));
    const before = ordered.find((option) => option.id === next[index - 1]);
    const after = ordered.find((option) => option.id === next[index + 1]);
    setDragOrder(next);
    save({ id: String(active.id), position: between(before?.position, after?.position) }).finally(
      () => setDragOrder(null),
    );
  };

  const addButton = (
    <OptionDialog
      kind={kind}
      trigger={
        <Button variant="ghost" size="icon" aria-label={`Add ${text.noun}`}>
          <PlusIcon className="size-4" />
        </Button>
      }
    />
  );

  return (
    <SettingsSection title={text.title} description={text.description} actions={addButton}>
      <ListStates
        isPending={query.isPending}
        error={query.error}
        onRetry={() => void query.refetch()}
        isEmpty={options.length === 0}
        empty={{
          icon: text.icon,
          title: text.empty,
          action: (
            <OptionDialog
              kind={kind}
              trigger={
                <Button variant="outline" size="sm">
                  <PlusIcon className="size-4" />
                  Add {text.noun}
                </Button>
              }
            />
          ),
        }}
      >
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext
            items={ordered.map((option) => option.id)}
            strategy={verticalListSortingStrategy}
          >
            <List>
              {ordered.map((option) => (
                <OptionRow key={option.id} kind={kind} option={option} />
              ))}
            </List>
          </SortableContext>
        </DndContext>
      </ListStates>
    </SettingsSection>
  );
}

function between(before: number | undefined, after: number | undefined) {
  if (before === undefined && after === undefined) return 0;
  if (before === undefined) return (after ?? 0) - 1;
  if (after === undefined) return before + 1;
  return (before + after) / 2;
}

function OptionRow({ kind, option }: { kind: TKind; option: TOption }) {
  const { remove } = useOptions(kind);
  const sortable = useSortable({ id: option.id });
  const { transform, transition, isDragging } = sortable;
  const count = option.customerCount;
  return (
    <div
      ref={sortable.setNodeRef}
      role="listitem"
      data-dragging={isDragging || undefined}
      style={{
        transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
        transition,
      }}
      className="bg-card data-dragging:bg-accent relative flex h-9 w-full items-center gap-2 border-b px-2 text-sm last:border-b-0 data-dragging:z-10"
    >
      <Button
        ref={sortable.setActivatorNodeRef}
        variant="ghost"
        size="icon"
        aria-label={`Reorder ${option.name}`}
        className="text-muted-foreground cursor-grab"
        {...sortable.attributes}
        {...sortable.listeners}
      >
        <GripVerticalIcon className="size-4" />
      </Button>
      <OptionDot color={option.color} />
      <span className="truncate">{option.name}</span>
      {option.description && (
        <span className="text-muted-foreground hidden truncate text-xs sm:inline">
          {option.description}
        </span>
      )}
      <span className="text-muted-foreground dank:type-label-small ml-auto shrink-0 text-xs">
        {count} {count === 1 ? "customer" : "customers"}
      </span>
      <div className="flex shrink-0 items-center gap-2">
        <OptionDialog
          kind={kind}
          option={option}
          trigger={
            <Button variant="ghost" size="icon" aria-label={`Edit ${option.name}`}>
              <PencilIcon className="size-4" />
            </Button>
          }
        />
        <ConfirmDialog
          trigger={
            <Button variant="ghost" size="icon" aria-label={`Delete ${option.name}`}>
              <Trash2Icon className="size-4" />
            </Button>
          }
          title={`Delete ${option.name}`}
          description={
            count > 0
              ? `${count} ${count === 1 ? "customer is" : "customers are"} left without a ${copy[kind].noun}.`
              : `No customer has this ${copy[kind].noun}.`
          }
          confirmLabel={`Delete ${copy[kind].noun}`}
          onConfirm={() => remove(option.id)}
        />
      </div>
    </div>
  );
}

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(64, "Keep the name under 64 characters."),
  color: z.string(),
  description: z.string().max(255, "Keep the description under 255 characters."),
});

function OptionDialog({
  kind,
  option,
  trigger,
}: {
  kind: TKind;
  option?: TOption;
  trigger: ReactElement;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const noun = copy[kind].noun;
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{option ? `Edit ${noun}` : `Add ${noun}`}</DialogTitle>
        </DialogHeader>
        {isOpen && <OptionForm kind={kind} option={option} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function OptionForm({
  kind,
  option,
  onDone,
}: {
  kind: TKind;
  option?: TOption;
  onDone: () => void;
}) {
  const { save } = useOptions(kind);
  const form = useAppForm({
    defaultValues: {
      name: option?.name ?? "",
      color: option?.color ?? PRESET_COLORS[0],
      description: option?.description ?? "",
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      await save({
        id: option?.id,
        name: parsed.name,
        color: parsed.color,
        description: parsed.description.trim() || null,
      });
      onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
      <form.AppField name="name" children={(field) => <field.TextField label="Name" autoFocus />} />
      <form.AppField
        name="description"
        children={(field) => <field.TextareaField label="Description" rows={2} />}
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
          <form.SubmitButton size="sm">
            {option ? "Save" : `Add ${copy[kind].noun}`}
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}
