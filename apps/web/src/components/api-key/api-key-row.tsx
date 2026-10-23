import type { APIKey } from "@/api/account.gen";
import { describeAccess, scopeLevel, scopeLevelItems } from "@/components/api-key/helpers";
import { KeySquareIcon, Trash2Icon } from "@/components/icons";
import { ListRow, ListRowText } from "@/components/layout/list";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useDeleteApiKeyMutation } from "@/lib/queries/api-keys";

/** How a key or connected app reaches Ricrac: its permissions, with its workspaces on hover. */
export function AccessBadge({
  access,
}: {
  access: { scopes: string[]; allWorkspaces: boolean; workspaces: string[] };
}) {
  const level = scopeLevel(access.scopes);
  const label = scopeLevelItems.find((item) => item.value === level)?.label ?? level;
  return (
    <SimpleTooltip content={describeAccess(access)}>
      <Badge variant="muted">{label}</Badge>
    </SimpleTooltip>
  );
}

export function ApiKeyRow({ apiKey }: { apiKey: APIKey }) {
  const { mutateAsync: deleteKey } = useDeleteApiKeyMutation();
  return (
    <ListRow
      actions={
        <>
          <span className="text-muted-foreground hidden text-xs md:inline">
            {apiKey.lastUsedAt ? (
              <>
                Used <RelativeTime value={apiKey.lastUsedAt} />
              </>
            ) : (
              "Never used"
            )}
          </span>
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Revoke ${apiKey.label}`}>
                <Trash2Icon className="size-4" />
              </Button>
            }
            title="Revoke API key"
            description={`Anything using ${apiKey.label} stops working immediately.`}
            confirmLabel="Revoke key"
            onConfirm={() => deleteKey(apiKey.id)}
          />
        </>
      }
    >
      <KeySquareIcon className="text-muted-foreground dank:size-5 size-4" />
      <ListRowText
        primary={apiKey.label}
        secondary={<span className="font-mono">{apiKey.keyHint}</span>}
      />
      <AccessBadge access={apiKey} />
    </ListRow>
  );
}
