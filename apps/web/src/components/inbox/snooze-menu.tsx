import { AlarmClockIcon, BellRingIcon } from "lucide-react";
import { snoozeOptions } from "@/components/inbox/snooze";
import { useNow } from "@/components/providers/now-provider";
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
import { Kbd } from "@/components/ui/kbd";
import { formatDateTime } from "@/lib/helpers/dates";

type TProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSnooze: (until: Date) => void;
  /** Offered when it is snoozed already. */
  onUnsnooze?: () => void;
};

export function SnoozeMenu({ open, onOpenChange, onSnooze, onUnsnooze }: TProps) {
  const now = useNow();
  const options = snoozeOptions(new Date(now));
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon" aria-label="Snooze">
            <AlarmClockIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-80">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="flex items-center gap-2">
            <span className="flex-1">Snooze until</span>
            <Kbd>H</Kbd>
          </DropdownMenuLabel>
          {options.map((option) => (
            <DropdownMenuItem key={option.id} onClick={() => onSnooze(option.until)}>
              <span className="flex-1">{option.label}</span>
              <span className="text-muted-foreground text-xs">{formatDateTime(option.until)}</span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
        {onUnsnooze && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onUnsnooze}>
              <BellRingIcon className="size-4" />
              Unsnooze now
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
