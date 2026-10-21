import { useQuery } from "@tanstack/react-query";
import { type LinkProps, useNavigate } from "@tanstack/react-router";
import {
  BoxIcon,
  CheckIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  ListTodoIcon,
  LogOutIcon,
  MessagesSquareIcon,
  MonitorIcon,
  MoonIcon,
  SettingsIcon,
  SquarePenIcon,
  SunIcon,
  TargetIcon,
  TriangleAlertIcon,
  UserCircleIcon,
  UserPlusIcon,
  UsersRoundIcon,
} from "lucide-react";
import { useTheme } from "next-themes";
import { ReactNode, useState } from "react";

import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { ShortcutKeys } from "@/components/command-panel/shortcut-keys";
import { ShortcutsDialog } from "@/components/command-panel/shortcuts-dialog";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import type { TTheme } from "@/components/providers/themes";
import {
  searchHitContext,
  searchHitHeadings,
  searchHitLink,
  SearchHitIcon,
  Snippet,
} from "@/components/search/search-hits";
import { useSearch } from "@/components/search/use-search";
import { useCreateTeamStore } from "@/components/settings/create-team-store";
import { useInviteStore } from "@/components/settings/invite-store";
import { TeamIcon } from "@/components/shared/team-icon";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useTeams } from "@/components/workspace/teams-provider";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { GO_TO_KEYS } from "@/lib/hotkeys/shortcuts";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useHotkeySequenceScope } from "@/lib/hotkeys/use-hotkey-sequences";
import { meQuery, signOut } from "@/lib/queries/me";
import type { TSearchHit } from "@/lib/queries/search";

type TDestination = keyof typeof GO_TO_KEYS;

/** The pages G then a letter, and the panel's Go to group, lead to. */
function useDestinations(): Record<
  TDestination,
  { label: string; icon: ReactNode; link: LinkProps }
> {
  const { workspace, isAdmin } = useWorkspace();
  const params = { workspace: workspace.urlKey };
  return {
    inbox: { label: "Inbox", icon: <InboxIcon />, link: { to: "/$workspace/inbox", params } },
    myIssues: {
      label: "My issues",
      icon: <UserCircleIcon />,
      link: { to: "/$workspace/my-issues", params },
    },
    issues: {
      label: "All issues",
      icon: <ListTodoIcon />,
      link: { to: "/$workspace/issues", params },
    },
    projects: {
      label: "Projects",
      icon: <BoxIcon />,
      link: { to: "/$workspace/projects", params },
    },
    views: { label: "Views", icon: <LayersIcon />, link: { to: "/$workspace/views", params } },
    initiatives: {
      label: "Initiatives",
      icon: <TargetIcon />,
      link: { to: "/$workspace/initiatives", params },
    },
    documents: {
      label: "Documents",
      icon: <FileTextIcon />,
      link: { to: "/$workspace/documents", params },
    },
    forum: {
      label: "Forum",
      icon: <MessagesSquareIcon />,
      link: { to: "/$workspace/forum", params },
    },
    settings: {
      label: "Settings",
      icon: <SettingsIcon />,
      link: { to: isAdmin ? "/$workspace/settings" : "/$workspace/settings/account", params },
    },
  };
}

/** The command panel (Mod+K or /), the shortcuts help (?) and G-then-letter navigation. */
export function CommandPanel() {
  const isOpen = useCommandPanelStore((state) => state.isOpen);
  const setOpen = useCommandPanelStore((state) => state.setOpen);
  const setShortcutsOpen = useCommandPanelStore((state) => state.setShortcutsOpen);
  const navigate = useNavigate();
  const destinations = useDestinations();

  useHotkeyScope([
    { hotkey: "Mod+K", callback: () => setOpen(true), ignoreInputs: false },
    { hotkey: "/", callback: () => setOpen(true) },
    { hotkey: "Shift+/", callback: () => setShortcutsOpen(true) },
  ]);
  useHotkeyScope([{ hotkey: "Mod+K", callback: () => setOpen(false), ignoreInputs: false }], {
    enabled: isOpen,
    modal: true,
  });
  useHotkeySequenceScope(
    (Object.keys(GO_TO_KEYS) as TDestination[]).map((destination) => ({
      sequence: ["G", GO_TO_KEYS[destination]],
      callback: () => void navigate(destinations[destination].link),
    })),
  );

  return (
    <>
      <Dialog open={isOpen} onOpenChange={setOpen}>
        <DialogContent
          variant="styleless"
          hideXButton
          animate={false}
          className="mt-16 mb-auto w-2xl max-w-full self-start sm:mt-24"
        >
          <DialogTitle className="sr-only">Command panel</DialogTitle>
          {isOpen && <CommandPanelContent onDone={() => setOpen(false)} />}
        </DialogContent>
      </Dialog>
      <ShortcutsDialog />
    </>
  );
}

type TAction = {
  id: string;
  label: string;
  icon: ReactNode;
  keywords?: string[];
  shortcut?: { keys: string[]; then?: boolean };
  isActive?: boolean;
  run: () => void;
};

function CommandPanelContent({ onDone }: { onDone: () => void }) {
  const [query, setQuery] = useState("");
  const navigate = useNavigate();
  const { theme: currentTheme, setTheme } = useTheme();
  const { workspace, canEdit } = useWorkspace();
  const { myTeams } = useTeams();
  const { data: me } = useQuery(meQuery);
  const openNewIssue = useNewIssueStore((state) => state.open);
  const openNewProject = useNewProjectStore((state) => state.open);
  const openCreateTeam = useCreateTeamStore((state) => state.setOpen);
  const openInvite = useInviteStore((state) => state.setOpen);
  const setShortcutsOpen = useCommandPanelStore((state) => state.setShortcutsOpen);
  const destinations = useDestinations();
  const search = useSearch(query);

  const go = (link: LinkProps) => {
    onDone();
    void navigate(link);
  };
  const run = (fn: () => void) => () => {
    onDone();
    fn();
  };

  const create: TAction[] = canEdit
    ? [
        {
          id: "new-issue",
          label: "New issue",
          icon: <SquarePenIcon />,
          keywords: ["create", "add"],
          shortcut: { keys: ["C"] },
          run: run(() => openNewIssue()),
        },
        {
          id: "new-project",
          label: "New project",
          icon: <BoxIcon />,
          keywords: ["create", "add"],
          run: run(() => openNewProject()),
        },
        {
          id: "new-team",
          label: "New team",
          icon: <UsersRoundIcon />,
          keywords: ["create", "add"],
          run: run(() => openCreateTeam(true)),
        },
        {
          id: "invite",
          label: "Invite people",
          icon: <UserPlusIcon />,
          keywords: ["member", "teammate", "add", "email"],
          run: run(() => openInvite(true)),
        },
      ]
    : [];

  const goTo: TAction[] = (Object.keys(GO_TO_KEYS) as TDestination[])
    .filter((destination) => canEdit || destination !== "initiatives")
    .map((destination) => {
      const { label, icon, link } = destinations[destination];
      return {
        id: `go-${destination}`,
        label: `Go to ${label.toLowerCase()}`,
        icon,
        keywords: ["navigate", "open"],
        shortcut: { keys: ["G", GO_TO_KEYS[destination]], then: true },
        run: () => go(link),
      };
    });

  const teams: TAction[] = myTeams.map((team) => ({
    id: `team-${team.id}`,
    label: `${team.name} issues`,
    keywords: [team.key, "team"],
    icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
    run: () =>
      go({
        to: "/$workspace/team/$key/issues",
        params: { workspace: workspace.urlKey, key: team.key },
      }),
  }));

  const workspaces: TAction[] = (me?.workspaces ?? [])
    .filter((item) => item.id !== workspace.id)
    .map((item) => ({
      id: `workspace-${item.id}`,
      label: `Switch to ${item.name}`,
      keywords: [item.urlKey, "workspace"],
      icon: <WorkspaceIcon name={item.name} logoUrl={item.logoUrl} />,
      run: () => go({ to: "/$workspace", params: { workspace: item.urlKey } }),
    }));

  const themeAction = (theme: TTheme, label: string, icon: ReactNode): TAction => ({
    id: `theme-${theme}`,
    label,
    icon,
    keywords: ["theme", "appearance", "mode"],
    isActive: currentTheme === theme,
    run: run(() => setTheme(theme)),
  });
  const preferences: TAction[] = [
    themeAction("light", "Light theme", <SunIcon />),
    themeAction("dark", "Dark theme", <MoonIcon />),
    themeAction("system", "System theme", <MonitorIcon />),
    {
      id: "shortcuts",
      label: "Keyboard shortcuts",
      icon: <KeyboardIcon />,
      keywords: ["hotkeys", "keys", "help"],
      shortcut: { keys: ["?"] },
      run: run(() => setShortcutsOpen(true)),
    },
    {
      id: "sign-out",
      label: "Sign out",
      icon: <LogOutIcon />,
      keywords: ["log out", "logout"],
      run: run(() => void signOut()),
    },
  ];

  return (
    <Command loop className="h-auto sm:max-h-120">
      <CommandInput
        autoFocus
        value={query}
        onValueChange={setQuery}
        showSpinner={search.isPending}
        placeholder="Search issues, projects, documents, discussions… or type a command"
      />
      <ScrollArea className="min-h-0">
        <CommandList>
          <CommandEmpty>{search.isPending ? "Searching…" : "Nothing found"}</CommandEmpty>
          {search.error && (
            <div
              role="alert"
              className="text-destructive flex items-center gap-2 px-3 py-2 text-sm"
            >
              <TriangleAlertIcon className="size-4" />
              Search failed: {search.error.message}
            </div>
          )}
          <SearchGroups hits={search.hits} query={query} onOpen={go} />
          <ActionGroup heading="Create" actions={create} />
          <ActionGroup heading="Go to" actions={goTo} />
          <ActionGroup heading="Teams" actions={teams} />
          <ActionGroup heading="Workspaces" actions={workspaces} />
          <ActionGroup heading="Preferences" actions={preferences} />
        </CommandList>
      </ScrollArea>
    </Command>
  );
}

const HIT_ORDER: TSearchHit["type"][] = [
  "issue",
  "project",
  "initiative",
  "document",
  "discussion",
  "comment",
];

function SearchGroups({
  hits,
  query,
  onOpen,
}: {
  hits: TSearchHit[];
  query: string;
  onOpen: (link: LinkProps) => void;
}) {
  const { workspace } = useWorkspace();
  return HIT_ORDER.map((type) => {
    const ofType = hits.filter((hit) => hit.type === type);
    if (ofType.length === 0) return null;
    return (
      <CommandGroup key={type} heading={searchHitHeadings[type]}>
        {ofType.map((hit) => {
          const context = searchHitContext(hit);
          return (
            <CommandItem
              key={`${hit.type}-${hit.id}`}
              value={`search-${hit.type}-${hit.id}`}
              keywords={[query]}
              onSelect={() => onOpen(searchHitLink(hit, workspace.urlKey))}
            >
              <SearchHitIcon hit={hit} />
              {(hit.type === "issue" || hit.type === "comment") && (
                <span className="text-muted-foreground shrink-0 font-mono text-xs">
                  {hit.identifier}
                </span>
              )}
              <span className="min-w-0 truncate">
                {hit.type === "comment" ? <Snippet text={hit.snippet} /> : hit.title}
              </span>
              {context && (
                <span className="text-muted-foreground ml-auto shrink-0 pl-2 text-xs">
                  {context}
                </span>
              )}
            </CommandItem>
          );
        })}
      </CommandGroup>
    );
  });
}

function ActionGroup({ heading, actions }: { heading: string; actions: TAction[] }) {
  if (actions.length === 0) return null;
  return (
    <CommandGroup heading={heading}>
      {actions.map((action) => (
        <CommandItem
          key={action.id}
          value={action.id}
          keywords={[action.label, ...(action.keywords ?? [])]}
          onSelect={action.run}
          className="[&>svg]:text-muted-foreground [&>svg]:size-4"
        >
          {action.icon}
          <span className="min-w-0 truncate">{action.label}</span>
          {action.isActive && <CheckIcon aria-label="Current" className="ml-auto" />}
          {action.shortcut && (
            <span className="ml-auto">
              <ShortcutKeys keys={action.shortcut.keys} then={action.shortcut.then} />
            </span>
          )}
        </CommandItem>
      ))}
    </CommandGroup>
  );
}
