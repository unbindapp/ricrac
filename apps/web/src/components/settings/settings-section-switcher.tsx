import { Link, type LinkProps } from "@tanstack/react-router";

import { BuildingIcon, type TIcon, UserIcon } from "@/components/icons/symbols.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export type TSettingsSection = "account" | "workspace";

type TProps = {
  current: TSettingsSection;
};

export function SettingsSectionSwitcher({ current }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  if (!canEdit) return null;
  const params = { workspace: workspace.urlKey };
  return (
    <nav
      aria-label="Settings sections"
      className="bg-surface-container-highest grid h-14 shrink-0 grid-cols-2 gap-1 rounded-full p-1"
    >
      <SectionLink
        link={{ to: "/$workspace/settings/account", params }}
        icon={UserIcon}
        label="Account"
        isCurrent={current === "account"}
      />
      <SectionLink
        link={{ to: "/$workspace/settings", params }}
        icon={BuildingIcon}
        label="Workspace"
        isCurrent={current === "workspace"}
      />
    </nav>
  );
}

type TSectionLinkProps = {
  link: LinkProps;
  icon: TIcon;
  label: string;
  isCurrent: boolean;
};

function SectionLink({ link, icon: Icon, label, isCurrent }: TSectionLinkProps) {
  return (
    <Link
      {...link}
      aria-current={isCurrent || undefined}
      data-current={isCurrent || undefined}
      className="text-muted-foreground has-hover:hover:bg-surface-container-high has-hover:hover:text-foreground data-current:bg-secondary data-current:text-secondary-foreground focus-visible:outline-primary ease-emphasized flex items-center justify-center gap-2 rounded-full text-base font-medium transition-colors duration-200 focus-visible:outline-2 focus-visible:-outline-offset-2"
    >
      <Icon className="size-5" />
      {label}
    </Link>
  );
}
