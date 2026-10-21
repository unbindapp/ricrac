import { type LinkProps } from "@tanstack/react-router";
import { type LucideIcon } from "lucide-react";
import { ReactNode, useState } from "react";

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
  "hit-gap-y h-9 w-full justify-start px-3 font-normal [&_svg]:text-muted-foreground";

export const sidebarActiveItemClassName =
  "data-[status=active]:bg-secondary data-[status=active]:text-secondary-foreground data-[status=active]:font-semibold data-[status=active]:[&_svg]:text-secondary-foreground";

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
