import * as React from "react";

import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";
import { cva, VariantProps } from "class-variance-authority";

const inputBoxClassName =
  "disabled:has-hover:hover:ring-0 read-only:has-hover:hover:ring-0 read-only:focus:ring-0 has-hover:hover:ring-1 has-hover:hover:ring-border data-staged:has-hover:hover:ring-change/7-10 data-staged:has-hover:hover:focus-visible:ring-change/8-10 data-staged:focus-visible:ring-change/8-10 has-hover:hover:focus-visible:ring-primary/8-10 w-full rounded-md border bg-input focus-visible:ring-1 focus-visible:ring-primary/8-10 data-staged:bg-change/2-10 data-staged:border-change/5-10";

const inputVariants = cva(
  "flex h-8 px-2.5 text-sm py-1.5 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground/9-10 focus-visible:outline-hidden disabled:cursor-not-allowed data-staged:text-change data-staged:placeholder:text-change/8-10",
  {
    variants: {
      variant: {
        default: "",
      },
      framed: {
        true: "min-w-0 flex-1 bg-transparent",
        false: inputBoxClassName,
      },
      fadeOnDisabled: {
        default: "disabled:opacity-50",
        false: "",
      },
    },
    defaultVariants: {
      variant: "default",
      framed: false,
      fadeOnDisabled: "default",
    },
  },
);

// State colors come from [data-text-field] in themes/dank/inputs.css
export const dankTextFieldVariants = cva(
  "min-w-0 bg-transparent text-base text-m3-on-surface caret-(--text-field-caret) outline-none placeholder:text-m3-on-surface-variant disabled:cursor-not-allowed disabled:text-m3-on-surface/38 disabled:placeholder:text-m3-on-surface/38",
  {
    variants: {
      box: {
        outlined:
          "w-full rounded-xs border border-(--text-field-outline) px-4 shadow-[inset_0_0_0_calc(var(--text-field-outline-width)-1px)_var(--text-field-outline)]",
        inField: "w-full rounded-xs px-4",
        none: "flex-1",
      },
    },
    defaultVariants: { box: "outlined" },
  },
);

const InOutlinedField = React.createContext(false);

export function useDankTextFieldBox(framed: boolean | null | undefined) {
  const inField = React.useContext(InOutlinedField);
  if (inField) return "inField";
  if (framed) return "none";
  return "outlined";
}

type TOutlinedFieldProps = {
  label: string;
  htmlFor: string;
  hideLabel?: boolean;
  invalid?: boolean;
  disabled?: boolean;
  /** Floats the label over controls that aren't text inputs. */
  populated?: boolean;
  className?: string;
  children: React.ReactNode;
};

export function OutlinedField({
  label,
  htmlFor,
  hideLabel,
  invalid,
  disabled,
  populated,
  className,
  children,
}: TOutlinedFieldProps) {
  return (
    <div
      data-slot="outlined-field"
      data-text-field=""
      data-invalid={invalid || undefined}
      data-disabled={disabled || undefined}
      data-populated={populated || undefined}
      className={cn("relative flex min-w-0", className)}
    >
      <InOutlinedField value={true}>{children}</InOutlinedField>
      <fieldset aria-hidden data-slot="outlined-field-outline">
        {!hideLabel && (
          <legend>
            <span>{label}</span>
          </legend>
        )}
      </fieldset>
      {hideLabel ? (
        <label htmlFor={htmlFor} className="sr-only">
          {label}
        </label>
      ) : (
        <label htmlFor={htmlFor} data-slot="outlined-field-label">
          {label}
        </label>
      )}
    </div>
  );
}

// The box of a framed input, the input inside it only holds the text
export const inputFrameClassName =
  "flex h-8 w-full items-stretch rounded-md border bg-input has-[input:disabled]:has-hover:hover:ring-0 has-hover:hover:ring-1 has-hover:hover:ring-border has-[input:focus-visible]:ring-1 has-[input:focus-visible]:ring-primary/8-10 has-hover:hover:has-[input:focus-visible]:ring-primary/8-10 data-staged:bg-change/2-10 data-staged:border-change/5-10 data-staged:has-hover:hover:ring-change/7-10 data-staged:has-[input:focus-visible]:ring-change/8-10 data-staged:has-hover:hover:has-[input:focus-visible]:ring-change/8-10";

export type InputProps = React.ComponentPropsWithRef<"input"> &
  VariantProps<typeof inputVariants> &
  InputLayout & {
    Icon?: React.FC<{ className?: string }>;
    classNameIcon?: string;
    hasChanges?: boolean;
  };

type InputLayout =
  | {
      layout: "label-included";
      inputTitle: string;
    }
  | {
      layout?: never;
      inputTitle?: never;
    };

function Input({
  className,
  variant,
  framed,
  fadeOnDisabled,
  inputTitle,
  layout,
  type,
  Icon,
  hasChanges,
  ...props
}: InputProps) {
  const isDank = useUiTheme().id === "dank";
  const dankBox = useDankTextFieldBox(framed);
  const generatedId = React.useId();

  if (isDank && layout === "label-included") {
    const id = props.id ?? generatedId;
    return (
      <OutlinedField
        label={inputTitle}
        htmlFor={id}
        invalid={props["aria-invalid"] === true || props["aria-invalid"] === "true"}
        disabled={props.disabled}
        className={cn(framed && "flex-1")}
      >
        <input
          type={type}
          data-slot="input"
          data-text-field=""
          className={cn(dankTextFieldVariants({ box: "inField" }), "h-14", className)}
          {...props}
          id={id}
          placeholder={props.placeholder || " "}
        />
      </OutlinedField>
    );
  }

  if (isDank) {
    return (
      <input
        type={type}
        data-slot="input"
        data-text-field=""
        className={cn(
          dankTextFieldVariants({ box: dankBox }),
          dankBox === "none" ? "flex h-9" : "h-14",
          className,
        )}
        {...props}
        placeholder={dankBox === "inField" ? props.placeholder || " " : props.placeholder}
      />
    );
  }

  if (layout === "label-included") {
    return (
      <div className={cn("relative", framed && "min-w-0 flex-1", className)}>
        <input
          type={type}
          data-staged={hasChanges || undefined}
          className={cn(
            inputVariants({
              variant,
              framed,
              fadeOnDisabled,
            }),
            "peer relative h-11 pt-4.5 pb-1.5",
            className,
          )}
          {...props}
          placeholder=" "
        />
        <label className="text-muted-foreground pointer-events-none absolute top-1/2 left-3.25 flex w-full origin-top-left -translate-y-[calc(100%-0.1rem)] scale-75 items-center justify-start gap-2 overflow-hidden leading-tight font-medium transition peer-placeholder-shown:-translate-y-1/2 peer-placeholder-shown:scale-100 peer-focus:-translate-y-[calc(100%-0.1rem)] peer-focus:scale-75">
          {Icon && <Icon className="size-4 shrink-0" />}
          <p className="min-w-0 shrink truncate">{inputTitle}</p>
        </label>
      </div>
    );
  }

  return (
    <input
      type={type}
      data-staged={hasChanges || undefined}
      className={cn(
        inputVariants({
          variant,
          framed,
          fadeOnDisabled,
        }),
        className,
      )}
      {...props}
    />
  );
}

export { Input };
