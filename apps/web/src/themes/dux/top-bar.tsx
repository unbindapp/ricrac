import { Link } from "@tanstack/react-router";

import { useIsAssistantAvailable } from "@/components/assistant/use-assistant-available";
import { ClaudeIcon } from "@/components/icons/claude";
import { InboxIcon, MenuIcon } from "@/components/icons";
import { useInboxUnreadCount } from "@/components/inbox/inbox-unread-badge";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useAssistantStore } from "@/lib/stores/assistant";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { AccountMenu } from "@/themes/dux/account-menu";
import { GlobalSearch } from "@/themes/dux/global-search";
import { DUX_SIDEBAR_ID } from "@/themes/dux/navigation";

/** The app bar across the window: the menu and brand on the left, search, Claude, inbox and account on the right. */
export function DuxTopBar() {
  return (
    <header data-slot="dux-app-bar">
      <NavigationToggle />
      <span data-slot="dux-brand">
        <img src="/dux.webp" alt="" />
        <span>dux</span>
      </span>
      <nav aria-label="Top actions" data-slot="dux-app-bar-actions">
        <GlobalSearch />
        <AskClaudeButton />
        <InboxButton />
        <AccountMenu />
      </nav>
    </header>
  );
}

function NavigationToggle() {
  const isSidebarHidden = useSidebarStore((state) => state.isCollapsed);
  const toggleSidebar = useSidebarStore((state) => state.toggle);
  const isDrawerOpen = useMobileNavStore((state) => state.isOpen);
  const setDrawerOpen = useMobileNavStore((state) => state.setOpen);
  const { size } = useDeviceSize();
  const isNarrow = size === "xs" || size === "sm";
  return (
    <SimpleTooltip content="Main menu" side="bottom">
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label="Main menu"
        aria-controls={isNarrow ? undefined : DUX_SIDEBAR_ID}
        aria-expanded={isNarrow ? isDrawerOpen : !isSidebarHidden}
        onClick={() => {
          if (isNarrow) return setDrawerOpen(!isDrawerOpen);
          toggleSidebar();
        }}
      >
        <MenuIcon />
      </Button>
    </SimpleTooltip>
  );
}

function AskClaudeButton() {
  const isAvailable = useIsAssistantAvailable();
  const isOpen = useAssistantStore((state) => state.isOpen);
  const toggle = useAssistantStore((state) => state.toggle);
  if (!isAvailable) return null;
  return (
    <SimpleTooltip content="Ask Claude" side="bottom">
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label="Ask Claude"
        aria-pressed={isOpen}
        data-slot="dux-ask-claude"
        onClick={toggle}
      >
        <ClaudeIcon />
      </Button>
    </SimpleTooltip>
  );
}

function InboxButton() {
  const { workspace } = useWorkspace();
  const unread = useInboxUnreadCount();
  const label = unread > 0 ? `Inbox, ${unread} unread` : "Inbox";
  return (
    <SimpleTooltip content="Inbox" side="bottom">
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={label}
        data-slot="dux-inbox"
        render={<Link to="/$workspace/inbox" params={{ workspace: workspace.urlKey }} />}
      >
        <InboxIcon />
        {unread > 0 && (
          <span aria-hidden data-slot="dux-badge">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </Button>
    </SimpleTooltip>
  );
}
