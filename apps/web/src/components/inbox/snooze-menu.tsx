import {
  AlarmClockIcon,
  BellRingIcon,
  CalendarIcon,
  ClockIcon,
  type LucideIcon,
  SunIcon,
} from "@/components/icons";
import { snoozeOptions, type TSnoozeOption } from "@/components/inbox/snooze";
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

const snoozeIcons: Record<TSnoozeOption["id"], LucideIcon> = {
  hour: ClockIcon,
  later: ClockIcon,
  tomorrow: SunIcon,
  "next-week": CalendarIcon,
};

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
          {options.map((option) => {
            const Icon = snoozeIcons[option.id];
            return (
              <DropdownMenuItem key={option.id} onClick={() => onSnooze(option.until)}>
                <Icon className="size-4" />
                <span className="dank:min-w-0 dank:truncate flex-1">{option.label}</span>
                <span className="text-muted-foreground dank:type-label-small dank:shrink-0 dank:whitespace-nowrap text-xs">
                  {formatDateTime(option.until)}
                </span>
              </DropdownMenuItem>
            );
          })}
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
