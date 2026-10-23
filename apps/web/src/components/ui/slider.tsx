import { Slider as SliderPrimitive } from "@base-ui/react/slider";
import type * as React from "react";

import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

export type SliderProps<Value extends number | readonly number[] = number | readonly number[]> =
  SliderPrimitive.Root.Props<Value> & {
    hasChanges?: boolean;
    /** Paints the track itself, like a gradient of the values, instead of filling up to the thumb. */
    trackStyle?: React.CSSProperties;
    /** What a screen reader calls each thumb. */
    thumbLabel?: string;
    /** Large is a thick track and a tall thumb, for a slider people drag a lot. */
    size?: "default" | "large";
  };

function Slider<Value extends number | readonly number[]>({
  className,
  defaultValue,
  value,
  min = 0,
  max = 100,
  onValueChange,
  onValueCommitted,
  hasChanges,
  trackStyle,
  thumbLabel,
  size = "default",
  ...props
}: SliderProps<Value>) {
  const isDank = useUiTheme().id === "dank";
  const _values = Array.isArray(value)
    ? value
    : Array.isArray(defaultValue)
      ? defaultValue
      : typeof (value ?? defaultValue) === "number"
        ? [value ?? defaultValue]
        : [min, max];

  // Base UI emits a plain number from pointer interactions on single-thumb
  // sliders even when `value`/`defaultValue` is an array (its keyboard path
  // emits arrays) - normalize so array in always means array out
  const expectsArray = Array.isArray(value ?? defaultValue);
  const normalize = (v: number | readonly number[]) =>
    (expectsArray && !Array.isArray(v) ? [v] : v) as Value extends number ? number : Value;

  return (
    <SliderPrimitive.Root
      data-slot="slider"
      defaultValue={defaultValue}
      value={value}
      min={min}
      max={max}
      onValueChange={
        onValueChange ? (v, details) => onValueChange(normalize(v), details) : undefined
      }
      onValueCommitted={
        onValueCommitted ? (v, details) => onValueCommitted(normalize(v), details) : undefined
      }
      thumbAlignment="edge"
      data-staged={hasChanges || undefined}
      data-size={size}
      className={cn(
        // The root is a plain wrapper around the control; center the control so
        // the track stays aligned when a flex row stretches the root
        "group/root flex data-[orientation=horizontal]:w-full data-[orientation=horizontal]:items-center data-[orientation=vertical]:h-full data-[orientation=vertical]:flex-col data-[orientation=vertical]:items-center",
        className,
      )}
      {...props}
    >
      {isDank ? (
        <SliderPrimitive.Control className="group/slider relative flex min-h-11 w-full touch-none items-center select-none active:cursor-grabbing has-hover:hover:cursor-grab has-hover:hover:active:cursor-grabbing data-disabled:pointer-events-none">
          <SliderPrimitive.Track
            data-slot="slider-track"
            data-painted={trackStyle ? true : undefined}
            style={(state) =>
              ({
                ...trackStyle,
                "--slider-fraction": max > min ? (state.values[0] - min) / (max - min) : 0,
              }) as React.CSSProperties
            }
            className="relative h-4 grow overflow-hidden rounded-full select-none"
          />
          {Array.from({ length: _values.length }, (_, index) => (
            <SliderPrimitive.Thumb
              data-slot="slider-thumb"
              key={index}
              getAriaLabel={thumbLabel ? () => thumbLabel : undefined}
              className="has-focus-visible:outline-m3-secondary block h-11 w-1 shrink-0 rounded-full select-none before:absolute before:-inset-x-3 before:inset-y-0 after:absolute after:inset-y-0 after:left-1/2 after:w-1 after:-translate-x-1/2 after:rounded-full after:bg-(--slider-handle) group-active/slider:after:w-0.5 active:cursor-grabbing has-focus-visible:outline-3 has-focus-visible:outline-offset-2 has-focus-visible:after:w-0.5"
            />
          ))}
        </SliderPrimitive.Control>
      ) : (
        <SliderPrimitive.Control className="group/slider relative flex w-full touch-none items-center select-none before:absolute before:top-1/2 before:left-1/2 before:h-full before:min-h-11 before:w-full before:min-w-11 before:-translate-1/2 active:cursor-grabbing has-hover:hover:cursor-grab has-hover:hover:active:cursor-grabbing data-disabled:pointer-events-none data-disabled:opacity-50 data-[orientation=vertical]:h-full data-[orientation=vertical]:min-h-44 data-[orientation=vertical]:w-auto data-[orientation=vertical]:flex-col">
          <SliderPrimitive.Track
            data-slot="slider-track"
            data-painted={trackStyle ? true : undefined}
            style={trackStyle}
            className="bg-border group-data-staged/root:bg-change/5-10 relative grow overflow-hidden rounded-full select-none data-[orientation=horizontal]:h-1.5 data-[orientation=horizontal]:w-full group-data-[size=large]/root:data-[orientation=horizontal]:h-3 data-[orientation=vertical]:h-full data-[orientation=vertical]:w-1.5"
          >
            <SliderPrimitive.Indicator
              data-slot="slider-range"
              className="bg-primary group-data-staged/root:bg-change select-none in-data-painted:hidden data-[orientation=horizontal]:h-full data-[orientation=vertical]:w-full"
            />
          </SliderPrimitive.Track>
          {Array.from({ length: _values.length }, (_, index) => (
            <SliderPrimitive.Thumb
              data-slot="slider-thumb"
              key={index}
              getAriaLabel={thumbLabel ? () => thumbLabel : undefined}
              className="bg-foreground shadow-shadow-color/shadow-opacity group-active/slider:ring-foreground/8-10 active:ring-foreground/8-10 ring-foreground/7-10 group-data-staged/root:bg-change group-data-staged/root:ring-change/7-10 group-data-staged/root:active:ring-change/8-10 group-data-staged/root:group-active/slider:ring-change/8-10 block size-4 shrink-0 rounded-full shadow-md transition-[color,box-shadow] select-none group-active/slider:ring-4 group-data-[size=large]/root:size-6 before:absolute before:-inset-x-3 before:-inset-y-1 active:cursor-grabbing active:ring-4 disabled:pointer-events-none disabled:opacity-50 has-hover:group-hover/slider:ring-6 has-hover:group-hover/slider:group-active/slider:ring-4 has-focus-visible:ring-6"
            />
          ))}
        </SliderPrimitive.Control>
      )}
    </SliderPrimitive.Root>
  );
}

export { Slider };
