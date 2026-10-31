import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTheme } from "next-themes";

import { type LucideIcon, MoonIcon, SettingsIcon, SunIcon, SunMoonIcon } from "@/components/icons";
import { UserAvatar } from "@/components/issues/pickers";
import { nextTheme, type TTheme } from "@/components/providers/themes";
import { Button } from "@/components/ui/button";
import { DropdownMenuLabel } from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceMenu } from "@/components/workspace/workspace-switcher";
import { meQuery } from "@/lib/queries/me";
import { GlobalSearch } from "@/themes/dank/global-search";

const themeModes: Record<TTheme, { label: string; icon: LucideIcon }> = {
  system: { label: "Follow system", icon: SunMoonIcon },
  light: { label: "Light", icon: SunIcon },
  dark: { label: "Dark", icon: MoonIcon },
};

const actionClassName = "size-12 shrink-0 rounded-full active:rounded-full popup-open:rounded-full";

/** The search bar centered at the top of the window, and the theme, settings and account top right. */
export function DankTopBar() {
  return (
    <header className="hidden h-20 shrink-0 md:block">
      <div className="fixed top-3 left-1/2 z-20 w-[min(35rem,calc(100vw-25rem))] -translate-x-1/2">
        <GlobalSearch />
      </div>
      <div className="fixed top-3 right-4 z-20 flex h-14 items-center gap-1">
        <ThemeModeButton />
        <SettingsButton />
        <AccountMenu />
      </div>
    </header>
  );
}

function ThemeModeButton() {
  const { theme, setTheme } = useTheme();
  const mode = themeModes[(theme as TTheme | undefined) ?? "system"] ?? themeModes.system;
  const label = `Theme: ${mode.label}`;
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={label}
        className={actionClassName}
        onClick={() => setTheme(nextTheme(theme))}
      >
        <mode.icon />
      </Button>
    </SimpleTooltip>
  );
}

function SettingsButton() {
  const { workspace, isAdmin } = useWorkspace();
  return (
    <SimpleTooltip content="Settings">
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label="Settings"
        className={actionClassName}
        render={
          <Link
            to={isAdmin ? "/$workspace/settings" : "/$workspace/settings/account"}
            params={{ workspace: workspace.urlKey }}
          />
        }
      >
        <SettingsIcon />
      </Button>
    </SimpleTooltip>
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
          className="popup-open:rounded-full ml-1 size-9 shrink-0 overflow-hidden rounded-full active:rounded-full [&>*]:size-9"
        >
          <UserAvatar user={me} size="lg" />
        </Button>
      }
    />
  );
}
