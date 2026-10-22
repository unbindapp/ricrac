import { Link, type LinkProps, useNavigate } from "@tanstack/react-router";
import { Fragment, ReactNode } from "react";

import { ChevronDownIcon, ChevronRightIcon, type TIcon } from "@/components/icons/symbols.gen";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export type TBreadcrumbMenuItem = { label: string; icon: TIcon; link: LinkProps };

export type TBreadcrumbItem = {
  label: string;
  icon?: ReactNode;
  link?: LinkProps;
  /** Pages next to this one, offered from a menu on the crumb. */
  menu?: TBreadcrumbMenuItem[];
};

export function Breadcrumb({ items }: { items: TBreadcrumbItem[] }) {
  if (items.length === 0) return null;
  const isCollapsedOnPhones = (index: number) => index < items.length - 2;
  return (
    <nav aria-label="Breadcrumb" className="min-w-0">
      <ol className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-sm">
        {items.map((item, index) => (
          <Fragment key={`${item.label}-${index}`}>
            {index > 0 && (
              <li
                aria-hidden
                data-collapsed={isCollapsedOnPhones(index - 1) || undefined}
                className="flex shrink-0 items-center max-sm:data-collapsed:hidden"
              >
                <ChevronRightIcon className="size-3.5" />
              </li>
            )}
            <li
              data-collapsed={isCollapsedOnPhones(index) || undefined}
              className="flex min-w-0 items-center max-sm:data-collapsed:hidden"
            >
              <BreadcrumbEntry item={item} isLast={index === items.length - 1} />
            </li>
          </Fragment>
        ))}
      </ol>
    </nav>
  );
}

function BreadcrumbEntry({ item, isLast }: { item: TBreadcrumbItem; isLast: boolean }) {
  const content = (
    <>
      {item.icon}
      <span className="truncate">{item.label}</span>
    </>
  );
  if (item.menu && !isLast) return <BreadcrumbMenu item={item} menu={item.menu} />;
  if (!item.link || isLast) {
    return (
      <span
        aria-current={isLast ? "page" : undefined}
        className="data-current:text-foreground flex min-w-0 items-center gap-1.5"
        data-current={isLast || undefined}
      >
        {content}
      </span>
    );
  }
  return (
    <Link
      {...item.link}
      className="has-hover:hover:text-foreground focus-visible:ring-primary/8-10 flex min-w-0 items-center gap-1.5 rounded-sm outline-hidden focus-visible:ring-1"
    >
      {content}
    </Link>
  );
}

function BreadcrumbMenu({ item, menu }: { item: TBreadcrumbItem; menu: TBreadcrumbMenuItem[] }) {
  const navigate = useNavigate();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`${item.label} pages`}
        className="has-hover:hover:text-foreground focus-visible:ring-primary data-popup-open:text-foreground flex min-w-0 items-center gap-1.5 rounded-sm outline-hidden focus-visible:ring-2"
      >
        {item.icon}
        <span className="truncate">{item.label}</span>
        <ChevronDownIcon className="size-3.5 shrink-0" />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-48">
        <DropdownMenuGroup>
          {menu.map((page) => (
            <DropdownMenuItem key={page.label} onClick={() => void navigate(page.link)}>
              <page.icon className="size-4" />
              {page.label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
