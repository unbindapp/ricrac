import { useQuery } from "@tanstack/react-query";

import { UserAvatar } from "@/components/issues/pickers";
import { Button } from "@/components/ui/button";
import { DropdownMenuLabel } from "@/components/ui/dropdown-menu";
import { cn } from "@/components/ui/utils";
import { WorkspaceMenu } from "@/components/workspace/workspace-switcher";
import { meQuery } from "@/lib/queries/me";
import { GlobalSearch } from "@/themes/dank/global-search";
import { useNavigationStore } from "@/themes/dank/navigation-store";

/** The search bar centered over pages, staying put while the rail expands. */
export function DankTopBar() {
  const isRailExpanded = useNavigationStore((state) => state.isExpanded);
  return (
    <header
      className={cn(
        "relative z-20 hidden shrink-0 justify-center pt-3 transition-transform duration-250 ease-[ease] md:flex",
        isRailExpanded && "-translate-x-24",
      )}
    >
      <GlobalSearch trailing={<AccountMenu />} />
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
