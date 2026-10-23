import { Switch as SwitchPrimitive } from "@base-ui/react/switch";

import { ToggleKnob } from "@/components/ui/toggle-knob";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TSwitchProps = Omit<SwitchPrimitive.Root.Props, "children"> & {
  size?: "sm" | "md";
};

function Switch({ className, checked, size = "sm", ...props }: TSwitchProps) {
  const isDank = useUiTheme().id === "dank";
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      checked={checked}
      className={cn(
        isDank
          ? "group/switch inline-flex shrink-0 cursor-pointer items-center rounded-full data-disabled:cursor-not-allowed"
          : "group/switch focus-visible:ring-primary/8-10 focus-visible:ring-offset-background inline-flex shrink-0 cursor-pointer items-center rounded-full outline-hidden focus-visible:ring-1 focus-visible:ring-offset-2 data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <SwitchKnob size={size} />
    </SwitchPrimitive.Root>
  );
}

function SwitchKnob({ size }: { size: "sm" | "md" }) {
  return (
    <SwitchPrimitive.Thumb
      render={(thumbProps, state) => (
        <span {...thumbProps}>
          <ToggleKnob checked={state.checked} size={size} />
        </span>
      )}
    />
  );
}

export { Switch };
