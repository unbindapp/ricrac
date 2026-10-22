import { useInfiniteQuery } from "@tanstack/react-query";
import { useState } from "react";

import {
  ChevronRightIcon,
  CircleCheckIcon,
  CircleXIcon,
  ClockIcon,
  EllipsisIcon,
  KeyRoundIcon,
  PauseIcon,
  PencilIcon,
  PlayIcon,
  SendIcon,
  Trash2Icon,
} from "@/components/icons/symbols.gen";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { resourceLabel } from "@/components/webhooks/webhook-form";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  type TWebhook,
  type TWebhookDelivery,
  useDeleteWebhookMutation,
  useRotateWebhookSecretMutation,
  useSaveWebhookMutation,
  useTestWebhookMutation,
  webhookDeliveriesQuery,
} from "@/lib/queries/webhooks";

type TProps = {
  webhook: TWebhook;
  onEdit: () => void;
  onSecret: (secret: string) => void;
};

function hostOf(url: string) {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function typesSummary(types: string[]) {
  if (types.length <= 2) return types.map(resourceLabel).join(", ");
  return `${resourceLabel(types[0])} and ${types.length - 1} more`;
}

export function WebhookRow({ webhook, onEdit, onSecret }: TProps) {
  const { workspace } = useWorkspace();
  const [isOpen, setIsOpen] = useState(false);
  const [confirm, setConfirm] = useState<"delete" | "rotate" | null>(null);
  const save = useSaveWebhookMutation(workspace.id);
  const test = useTestWebhookMutation(workspace.id);
  const { mutateAsync: remove } = useDeleteWebhookMutation(workspace.id);
  const { mutateAsync: rotate } = useRotateWebhookSecretMutation(workspace.id);
  const name = webhook.label || hostOf(webhook.url);

  const sendTest = () =>
    test.mutate(webhook.id, {
      onSuccess: (ping) =>
        toast.add(
          ping.delivered
            ? { type: "success", title: "Test delivered", description: `HTTP ${ping.statusCode}` }
            : {
                type: "error",
                title: "Test failed",
                description: ping.error ?? `HTTP ${ping.statusCode}`,
              },
        ),
    });

  return (
    <div role="listitem" className="flex w-full flex-col border-b last:border-b-0">
      <div className="flex w-full items-center gap-2 px-3 py-2">
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-expanded={isOpen}
          aria-label={isOpen ? "Hide deliveries" : "Show deliveries"}
          onClick={() => setIsOpen((open) => !open)}
          className="text-muted-foreground -ml-1 size-6"
        >
          <ChevronRightIcon
            data-open={isOpen || undefined}
            className="size-4 transition-transform data-open:rotate-90"
          />
        </Button>
        <span
          aria-hidden
          data-enabled={webhook.enabled || undefined}
          className="bg-muted-foreground/40 data-enabled:bg-success size-2 shrink-0 rounded-full"
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex min-w-0 items-center gap-2 text-sm">
            <span className="truncate font-medium">{name}</span>
            {!webhook.enabled && <Badge variant="muted">Disabled</Badge>}
          </div>
          <span className="text-muted-foreground truncate font-mono text-xs">{webhook.url}</span>
        </div>
        <SimpleTooltip content={webhook.resourceTypes.map(resourceLabel).join(", ")}>
          <span className="text-muted-foreground hidden shrink-0 text-xs sm:inline">
            {typesSummary(webhook.resourceTypes)}
          </span>
        </SimpleTooltip>
        {webhook.team && (
          <Badge variant="outline" className="hidden sm:inline-flex">
            {webhook.team.key}
          </Badge>
        )}
        <SimpleTooltip content="Send a test ping">
          <Button
            variant="ghost"
            size="icon"
            aria-label={`Send a test to ${name}`}
            isPending={test.isPending}
            disabled={!webhook.enabled}
            onClick={sendTest}
          >
            <SendIcon className="size-4" />
          </Button>
        </SimpleTooltip>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" aria-label={`Actions for ${name}`}>
                <EllipsisIcon className="size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem onClick={onEdit}>
              <PencilIcon className="size-4" />
              Edit webhook
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => save.mutate({ id: webhook.id, enabled: !webhook.enabled })}
            >
              {webhook.enabled ? <PauseIcon className="size-4" /> : <PlayIcon className="size-4" />}
              {webhook.enabled ? "Disable" : "Enable"}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setConfirm("rotate")}>
              <KeyRoundIcon className="size-4" />
              Rotate secret
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setConfirm("delete")}>
              <Trash2Icon className="size-4" />
              Delete webhook
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {isOpen && <Deliveries webhookId={webhook.id} />}
      <ConfirmDialog
        open={confirm === "delete"}
        onOpenChange={(open) => !open && setConfirm(null)}
        title={`Delete ${name}`}
        description="It stops receiving events and its queued deliveries are dropped."
        confirmLabel="Delete webhook"
        onConfirm={() => remove(webhook.id)}
      />
      <ConfirmDialog
        open={confirm === "rotate"}
        onOpenChange={(open) => !open && setConfirm(null)}
        title="Rotate the signing secret"
        description="Deliveries are signed with a new secret from now on; the receiver needs it to check them."
        confirmLabel="Rotate secret"
        variant="default"
        onConfirm={async () => onSecret((await rotate(webhook.id)).secret)}
      />
    </div>
  );
}

function Deliveries({ webhookId }: { webhookId: string }) {
  const { workspace } = useWorkspace();
  const query = useInfiniteQuery(webhookDeliveriesQuery(workspace.id, webhookId));
  const deliveries = query.data?.pages.flatMap((page) => page.nodes) ?? [];

  return (
    <div className="bg-muted/30 flex flex-col border-t px-3 py-2 pl-12">
      {query.isPending && <p className="text-muted-foreground py-1 text-xs">Loading deliveries…</p>}
      {query.error && (
        <p className="text-destructive py-1 text-xs">
          Couldn't load deliveries: {query.error.message}
        </p>
      )}
      {!query.isPending && !query.error && deliveries.length === 0 && (
        <p className="text-muted-foreground py-1 text-xs">
          Nothing sent in the last 14 days. Send a test to try it.
        </p>
      )}
      {deliveries.map((delivery) => (
        <DeliveryRow key={delivery.id} delivery={delivery} />
      ))}
      {query.hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          isPending={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
          className="self-start"
        >
          Show older
        </Button>
      )}
    </div>
  );
}

function DeliveryRow({ delivery }: { delivery: TWebhookDelivery }) {
  const state = delivery.deliveredAt ? "delivered" : delivery.failedAt ? "failed" : "pending";
  const Icon = { delivered: CircleCheckIcon, failed: CircleXIcon, pending: ClockIcon }[state];
  const attempts = delivery.attempts > 1 ? `${delivery.attempts} attempts` : null;
  const detail = [delivery.statusCode && `HTTP ${delivery.statusCode}`, attempts]
    .filter(Boolean)
    .join(" · ");
  return (
    <div className="flex h-8 items-center gap-2 text-xs">
      <Icon
        data-state={state}
        aria-label={state}
        className="text-muted-foreground data-[state=delivered]:text-success data-[state=failed]:text-destructive size-3.5 shrink-0"
      />
      <span className="w-40 shrink-0 truncate">
        {resourceLabel(delivery.type)} {delivery.action}
      </span>
      <span className="text-muted-foreground w-32 shrink-0 truncate">{detail}</span>
      <span
        title={delivery.error ?? undefined}
        className="text-muted-foreground min-w-0 flex-1 truncate"
      >
        {delivery.error ?? (state === "pending" && delivery.nextAttemptAt ? "Retrying soon" : "")}
      </span>
      <span className="text-muted-foreground shrink-0">
        <RelativeTime value={delivery.createdAt} />
      </span>
    </div>
  );
}
