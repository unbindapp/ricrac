import { CheckIcon } from "@/components/icons";
import { cn } from "@/components/ui/utils";

type TProps = {
  checked: boolean;
  hasChanges?: boolean;
  className?: string;
  size?: "sm" | "md";
};

export function ToggleKnob({ checked, hasChanges, size = "md", className }: TProps) {
  return (
    <div
      data-checked={checked || undefined}
      data-staged={hasChanges || undefined}
      data-size={size}
      className={cn(
        // The color is animated through currentColor: Chrome runs background-color transitions
        // on the compositor and can paint the start color for a frame when they end
        "group/knob text-muted-more-foreground data-checked:text-foreground data-staged:text-change/7-10 data-staged:data-checked:text-change dank:text-surface-container-highest dank:border-muted-more-foreground dank:data-checked:text-primary dank:data-checked:border-primary dank:data-staged:data-checked:text-change dank:data-staged:data-checked:border-change dank:h-8 dank:w-13 dank:border-2 dank:data-[size=sm]:h-6 dank:data-[size=sm]:w-10 relative h-5 w-9 shrink-0 rounded-full bg-current transition data-[size=sm]:h-4.5 data-[size=sm]:w-8",
        className,
      )}
    >
      <div className="bg-background dank:hidden absolute top-0.5 left-0.5 size-4 rounded-full transition group-data-checked/knob:translate-x-4 group-data-[size=sm]/knob:size-3.5 group-data-checked/knob:group-data-[size=sm]/knob:translate-x-3.5" />
      <div className="bg-muted-more-foreground group-data-checked/knob:bg-primary-foreground group-data-checked/knob:text-primary group-data-staged/knob:group-data-checked/knob:text-change ease-emphasized dank:flex absolute top-1/2 left-1.5 hidden size-4 -translate-y-1/2 items-center justify-center rounded-full transition-all duration-300 group-active/switch:left-0 group-active/switch:size-7 group-data-checked/knob:left-5.5 group-data-checked/knob:size-6 group-data-checked/knob:group-active/switch:left-5 group-data-checked/knob:group-active/switch:size-7 group-data-[size=sm]/knob:left-1 group-data-[size=sm]/knob:size-3 group-data-[size=sm]/knob:group-active/switch:left-0 group-data-[size=sm]/knob:group-active/switch:size-5 group-data-checked/knob:group-data-[size=sm]/knob:left-4.25 group-data-checked/knob:group-data-[size=sm]/knob:size-4.5 group-data-checked/knob:group-data-[size=sm]/knob:group-active/switch:left-4 group-data-checked/knob:group-data-[size=sm]/knob:group-active/switch:size-5">
        <CheckIcon className="size-3.5 opacity-0 transition-opacity group-data-checked/knob:opacity-100 group-data-[size=sm]/knob:size-3" />
      </div>
    </div>
  );
}
