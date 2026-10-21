import { Slider as SliderPrimitive } from "@base-ui/react/slider";

import type * as React from "react";

import { cn } from "@/components/ui/utils";

export type SliderProps<Value extends number | readonly number[] = number | readonly number[]> =
  SliderPrimitive.Root.Props<Value> & {
    hasChanges?: boolean;
    /** Paints the track itself, like a gradient of the values, instead of filling up to the thumb. */
    trackStyle?: React.CSSProperties;
    /** What a screen reader calls each thumb. */
    thumbLabel?: string;
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
  ...props
}: SliderProps<Value>) {
  const _values = Array.isArray(value)
    ? value
    : Array.isArray(defaultValue)
      ? defaultValue
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
      className={cn(
        // The root is a plain wrapper around the control; center the control so
        // the track stays aligned when a flex row stretches the root
        "group/root flex data-[orientation=horizontal]:w-full data-[orientation=horizontal]:items-center data-[orientation=vertical]:h-full data-[orientation=vertical]:flex-col data-[orientation=vertical]:items-center",
        className,
      )}
      {...props}
    >
      <SliderPrimitive.Control className="group/slider relative flex w-full touch-none items-center select-none before:absolute before:top-1/2 before:left-1/2 before:h-full before:min-h-11 before:w-full before:min-w-11 before:-translate-1/2 active:cursor-grabbing has-hover:hover:cursor-grab has-hover:hover:active:cursor-grabbing data-disabled:pointer-events-none data-disabled:opacity-50 data-[orientation=vertical]:h-full data-[orientation=vertical]:min-h-44 data-[orientation=vertical]:w-auto data-[orientation=vertical]:flex-col">
        <SliderPrimitive.Track
          data-slot="slider-track"
          data-painted={trackStyle ? true : undefined}
          style={trackStyle}
          className="bg-secondary group-data-staged/root:bg-change/5-10 relative grow rounded-full select-none data-[orientation=horizontal]:h-4 data-[orientation=horizontal]:w-full data-[orientation=vertical]:h-full data-[orientation=vertical]:w-4"
        >
          <SliderPrimitive.Indicator
            data-slot="slider-range"
            className="bg-primary group-data-staged/root:bg-change rounded-full select-none in-data-painted:opacity-0 data-[orientation=horizontal]:h-full data-[orientation=vertical]:w-full"
          />
        </SliderPrimitive.Track>
        {Array.from({ length: _values.length }, (_, index) => (
          <SliderPrimitive.Thumb
            data-slot="slider-thumb"
            key={index}
            getAriaLabel={thumbLabel ? () => thumbLabel : undefined}
            className="bg-primary group-data-staged/root:bg-change group-has-data-painted/slider:bg-foreground ease-emphasized has-focus-visible:outline-primary block h-11 w-1 shrink-0 rounded-full ring-6 ring-[color:var(--slider-gap,var(--color-card))] transition-[width] duration-200 select-none group-active/slider:w-0.5 active:cursor-grabbing disabled:pointer-events-none disabled:opacity-50 has-focus-visible:outline-2 has-focus-visible:outline-offset-4 data-[orientation=vertical]:h-1 data-[orientation=vertical]:w-11"
          />
        ))}
      </SliderPrimitive.Control>
    </SliderPrimitive.Root>
  );
}

export { Slider };
