import { CheckIcon } from "@/components/icons";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = {
  checked: boolean;
  hasChanges?: boolean;
  className?: string;
  size?: "sm" | "md";
};

export function ToggleKnob({ checked, hasChanges, size = "md", className }: TProps) {
  const isDank = useUiTheme().id === "dank";
  if (isDank)
    return <DankSwitchTrack checked={checked} hasChanges={hasChanges} className={className} />;
  return (
    <div
      data-checked={checked || undefined}
      data-staged={hasChanges || undefined}
      data-size={size}
      className={cn(
        // The color is animated through currentColor: Chrome runs background-color transitions
        // on the compositor and can paint the start color for a frame when they end
        "group/knob text-muted-more-foreground data-checked:text-foreground data-staged:text-change/7-10 data-staged:data-checked:text-change relative h-5 w-9 shrink-0 rounded-full bg-current transition data-[size=sm]:h-4.5 data-[size=sm]:w-8",
        className,
      )}
    >
      <div className="bg-background absolute top-0.5 left-0.5 size-4 rounded-full transition group-data-checked/knob:translate-x-4 group-data-[size=sm]/knob:size-3.5 group-data-checked/knob:group-data-[size=sm]/knob:translate-x-3.5" />
    </div>
  );
}

// Colors and handle geometry come from [data-slot="switch"] in themes/dank/inputs.css
function DankSwitchTrack({ checked, hasChanges, className }: Omit<TProps, "size">) {
  return (
    <div
      data-checked={checked || undefined}
      data-staged={hasChanges || undefined}
      className={cn(
        "group/knob data-staged:data-checked:text-change relative h-8 w-13 shrink-0 rounded-full border-2 border-(--switch-outline) bg-current text-(--switch-track) transition-[color,border-color]",
        className,
      )}
    >
      <div
        data-slot="switch-handle"
        className="absolute top-1/2 left-(--switch-handle-left) flex size-(--switch-handle-size) -translate-y-1/2 items-center justify-center rounded-full bg-current text-(--switch-handle) before:absolute before:top-1/2 before:left-1/2 before:size-10 before:-translate-1/2 before:rounded-full before:bg-(--switch-state-layer) before:opacity-(--switch-state-layer-opacity) before:transition-opacity"
      >
        <CheckIcon className="relative size-4 text-(--switch-icon) opacity-0 transition-opacity group-data-checked/knob:opacity-100" />
      </div>
    </div>
  );
}
