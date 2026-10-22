import { useNavigate } from "@tanstack/react-router";
import { ReactNode } from "react";

import type { Team } from "@/api/ops.gen";
import { BuildingIcon, CheckIcon, ChevronDownIcon, UserIcon } from "@/components/icons/symbols.gen";
import { TeamIcon } from "@/components/shared/team-icon";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export type TSettingsSection =
  { kind: "account" } | { kind: "workspace" } | { kind: "team"; key: string };

type TProps = {
  current: TSettingsSection;
};

export function SettingsSectionSwitcher({ current }: TProps) {
  const { workspace, isAdmin, canEdit } = useWorkspace();
  const { teams, teamByKey } = useTeams();
  const navigate = useNavigate();
  const params = { workspace: workspace.urlKey };
  const currentTeam = current.kind === "team" ? teamByKey(current.key) : undefined;
  const listedTeams = canEdit
    ? teams.filter((team) => team.isMember || team.id === currentTeam?.id)
    : [];

  const open = (section: TSettingsSection) => {
    switch (section.kind) {
      case "account":
        void navigate({ to: "/$workspace/settings/account", params });
        return;
      case "workspace":
        void navigate({ to: "/$workspace/settings", params });
        return;
      case "team":
        void navigate({
          to: "/$workspace/settings/team/$key",
          params: { ...params, key: section.key },
        });
        return;
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            aria-label="Switch settings section"
            className="border-primary bg-surface-container has-hover:hover:bg-surface-container-high data-popup-open:bg-surface-container-high h-14 w-full shrink-0 justify-start gap-3 rounded-full border-2 px-5 text-base"
          >
            <SectionIcon section={current} team={currentTeam} />
            <span className="min-w-0 flex-1 truncate text-left">
              {sectionLabel(current, currentTeam)}
            </span>
            <ChevronDownIcon className="text-muted-foreground size-5" />
          </Button>
        }
      />
      <DropdownMenuContent className="w-72">
        <DropdownMenuGroup>
          <SectionItem
            isCurrent={current.kind === "account"}
            onClick={() => open({ kind: "account" })}
          >
            <UserIcon className="size-4" />
            Account
          </SectionItem>
          {isAdmin && (
            <SectionItem
              isCurrent={current.kind === "workspace"}
              onClick={() => open({ kind: "workspace" })}
            >
              <BuildingIcon className="size-4" />
              Workspace
            </SectionItem>
          )}
        </DropdownMenuGroup>
        {listedTeams.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuLabel>Teams</DropdownMenuLabel>
              {listedTeams.map((team) => (
                <SectionItem
                  key={team.id}
                  isCurrent={team.id === currentTeam?.id}
                  onClick={() => open({ kind: "team", key: team.key })}
                >
                  <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />
                  {team.name}
                </SectionItem>
              ))}
            </DropdownMenuGroup>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

type TSectionItemProps = {
  isCurrent: boolean;
  onClick: () => void;
  children: ReactNode;
};

function SectionItem({ isCurrent, onClick, children }: TSectionItemProps) {
  return (
    <DropdownMenuItem onClick={onClick}>
      {children}
      <span className="flex-1" />
      {isCurrent && <CheckIcon className="size-4" />}
    </DropdownMenuItem>
  );
}

function SectionIcon({ section, team }: { section: TSettingsSection; team?: Team }) {
  switch (section.kind) {
    case "account":
      return <UserIcon className="text-primary size-5" />;
    case "workspace":
      return <BuildingIcon className="text-primary size-5" />;
    case "team":
      return <TeamIcon teamKey={section.key} icon={team?.icon} color={team?.color} />;
  }
}

function sectionLabel(section: TSettingsSection, team?: Team) {
  switch (section.kind) {
    case "account":
      return "Account";
    case "workspace":
      return "Workspace";
    case "team":
      return team?.name ?? section.key;
  }
}
