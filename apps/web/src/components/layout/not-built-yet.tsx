import { HammerIcon } from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";

export function NotBuiltYet() {
  return <EmptyState icon={HammerIcon} title="Not built yet" />;
}
