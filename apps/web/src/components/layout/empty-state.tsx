import { LucideIcon } from "lucide-react";
import { ReactNode } from "react";

import { ShapeBadge } from "@/components/ui/shape-badge";

type TProps = {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: ReactNode;
};

export function EmptyState({ icon: Icon, title, description, action }: TProps) {
  return (
    <div
      data-slot="empty-state"
      className="flex w-full flex-col items-center justify-center gap-2 px-4 py-12 text-center"
    >
      <ShapeBadge shape="cookie" className="text-tertiary-container size-14">
        <Icon className="text-tertiary-container-foreground size-5" />
      </ShapeBadge>
      <p className="text-sm font-medium">{title}</p>
      {description && <p className="text-muted-foreground max-w-sm text-sm">{description}</p>}
      {action && <div className="flex items-center gap-2 pt-2">{action}</div>}
    </div>
  );
}
