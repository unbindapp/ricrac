import { useQuery } from "@tanstack/react-query";

import { UserAvatar } from "@/components/issues/pickers";
import { Button } from "@/components/ui/button";
import { DropdownMenuLabel } from "@/components/ui/dropdown-menu";
import { WorkspaceMenu } from "@/components/workspace/workspace-switcher";
import { meQuery } from "@/lib/queries/me";
import { GlobalSearch } from "@/themes/dank/global-search";

/** The search bar, centered over pages beside the collapsed rail and fixed there while it expands. */
export function DankTopBar() {
  return (
    <header className="hidden h-17 shrink-0 md:block">
      <div className="fixed top-6 left-[calc(50%+3.125rem)] z-20 flex w-[calc(100vw-31.75rem)] max-w-160 -translate-x-1/2 justify-center">
        <GlobalSearch trailing={<AccountMenu />} />
      </div>
    </header>
  );
}

function AccountMenu() {
  const { data: me } = useQuery(meQuery);
  if (!me) return null;
  return (
    <WorkspaceMenu
      align="end"
      header={
        <DropdownMenuLabel className="flex flex-col gap-0.5">
          <span className="text-foreground truncate text-sm">{me.name}</span>
          <span className="truncate">{me.email}</span>
        </DropdownMenuLabel>
      }
      trigger={
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label="Account and workspaces"
          className="popup-open:rounded-full size-12 shrink-0 rounded-full active:rounded-full [&>*]:size-7.5"
        >
          <UserAvatar user={me} size="lg" />
        </Button>
      }
    />
  );
}
