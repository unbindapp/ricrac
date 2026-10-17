import { ReactNode } from "react";

type TProps = {
  children?: ReactNode;
  actions?: ReactNode;
};

export function PageToolbar({ children, actions }: TProps) {
  return (
    <div data-slot="page-toolbar" className="flex min-h-8 w-full items-center gap-2">
      <div className="scrollbar-hidden flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">
        {children}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-1">{actions}</div>}
    </div>
  );
}
