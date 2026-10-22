import {
  BugIcon,
  CircleHelpIcon,
  LightbulbIcon,
  MessageCircleIcon,
  PaletteIcon,
  SquareCheckBigIcon,
  type TIcon,
} from "@/components/icons/symbols.gen";
import { Badge } from "@/components/ui/badge";
import type { TThreadKind } from "@/lib/queries/discussions";

export const THREAD_KINDS = [
  "general",
  "bug",
  "design",
  "question",
  "idea",
  "task",
] as const satisfies readonly TThreadKind[];

export const threadKinds: Record<TThreadKind, { label: string; icon: TIcon }> = {
  general: { label: "General", icon: MessageCircleIcon },
  bug: { label: "Bug", icon: BugIcon },
  design: { label: "Design", icon: PaletteIcon },
  question: { label: "Question", icon: CircleHelpIcon },
  idea: { label: "Idea", icon: LightbulbIcon },
  task: { label: "Task", icon: SquareCheckBigIcon },
};

export function threadKindOf(kind: string): TThreadKind {
  return THREAD_KINDS.find((known) => known === kind) ?? "general";
}

export function ThreadKindBadge({ kind }: { kind: string }) {
  const { label, icon: Icon } = threadKinds[threadKindOf(kind)];
  return (
    <Badge variant="muted">
      <Icon />
      {label}
    </Badge>
  );
}
