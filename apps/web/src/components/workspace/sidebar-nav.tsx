import { type LinkProps } from "@tanstack/react-router";
import { ReactNode, useState } from "react";

import type { LucideIcon } from "@/components/icons";
import { Button, LinkButton } from "@/components/ui/button";

type TNavItemProps = {
  label: string;
  icon: LucideIcon;
  link: LinkProps;
  badge?: ReactNode;
  exact?: boolean;
  onNavigate?: () => void;
};

export const sidebarItemClassName =
  "hit-gap-y h-7.5 w-full justify-start px-2 font-normal [&_svg]:text-muted-foreground dank:h-9 dank:pr-3 dank:pl-3";

export const sidebarActiveItemClassName =
  "data-[status=active]:bg-accent dank:data-[status=active]:bg-secondary dank:data-[status=active]:text-secondary-foreground dank:data-[status=active]:font-semibold dank:data-[status=active]:[&_svg]:text-secondary-foreground";

export function SidebarNavItem({
  label,
  icon: Icon,
  link,
  badge,
  exact,
  onNavigate,
}: TNavItemProps) {
  return (
    <LinkButton
      {...link}
      activeOptions={{ exact: exact ?? false, includeSearch: false }}
      variant="ghost"
      size="sm"
      forceMinSize={false}
      onClick={onNavigate}
      className={`${sidebarItemClassName} ${sidebarActiveItemClassName}`}
    >
      <Icon className="size-4" />
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      {badge}
    </LinkButton>
  );
}

function Caret({ isOpen }: { isOpen: boolean }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 8 8"
      data-open={isOpen || undefined}
      className="size-2 shrink-0 -rotate-90 fill-current transition-transform data-open:rotate-0"
    >
      <path d="M1 2.5h6L4 6z" />
    </svg>
  );
}

export function SidebarSection({
  title,
  action,
  children,
  defaultOpen = true,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="group/section flex flex-col">
      <div className="flex items-center gap-1 pr-1">
        <Button
          variant="ghost"
          size="sm"
          forceMinSize={false}
          aria-expanded={isOpen}
          onClick={() => setIsOpen((open) => !open)}
          className="text-muted-foreground h-7 min-w-0 justify-start gap-1 px-2 text-xs"
        >
          <span className="truncate">{title}</span>
          <Caret isOpen={isOpen} />
        </Button>
        <span className="flex-1" />
        {action}
      </div>
      {isOpen && <div className="flex flex-col">{children}</div>}
    </div>
  );
}

export function SidebarCollapsible({
  label,
  icon,
  children,
  defaultOpen = true,
}: {
  label: string;
  icon: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="flex flex-col">
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-expanded={isOpen}
        onClick={() => setIsOpen((open) => !open)}
        className={sidebarItemClassName}
      >
        {icon}
        <span className="min-w-0 truncate text-left">{label}</span>
        <span className="text-muted-foreground flex">
          <Caret isOpen={isOpen} />
        </span>
      </Button>
      {isOpen && <div className="flex flex-col pl-5">{children}</div>}
    </div>
  );
}
