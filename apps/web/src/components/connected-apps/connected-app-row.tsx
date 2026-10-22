import type { ConnectedApp } from "@/api/account.gen";
import { AccessBadge } from "@/components/api-key/api-key-row";
import { BoxIcon, Trash2Icon } from "@/components/icons/symbols.gen";
import { ListRow, ListRowText } from "@/components/layout/list";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { useRevokeConnectedAppMutation } from "@/lib/queries/connected-apps";

export function ConnectedAppRow({ app }: { app: ConnectedApp }) {
  const { mutateAsync: revoke } = useRevokeConnectedAppMutation();
  return (
    <ListRow
      actions={
        <>
          <span className="text-muted-foreground hidden text-xs md:inline">
            Used <RelativeTime value={app.lastUsedAt} />
          </span>
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Revoke ${app.name}`}>
                <Trash2Icon className="size-4" />
              </Button>
            }
            title="Revoke access"
            description={`${app.name} loses access immediately and has to be connected again.`}
            confirmLabel="Revoke access"
            onConfirm={() => revoke(app.id)}
          />
        </>
      }
    >
      <BoxIcon className="text-muted-foreground size-4" />
      <ListRowText
        primary={app.name}
        secondary={
          <>
            Connected <RelativeTime value={app.createdAt} />
          </>
        }
      />
      <AccessBadge access={app} />
    </ListRow>
  );
}
