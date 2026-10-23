import * as React from "react";

import { dankTextFieldVariants, useDankTextFieldBox } from "@/components/ui/input";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  const isDank = useUiTheme().id === "dank";
  const dankBox = useDankTextFieldBox(false);

  if (isDank) {
    return (
      <textarea
        data-slot="textarea"
        data-text-field=""
        className={cn(dankTextFieldVariants({ box: dankBox }), "flex min-h-14 py-4", className)}
        {...props}
        placeholder={dankBox === "inField" ? props.placeholder || " " : props.placeholder}
      />
    );
  }

  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "bg-input placeholder:text-muted-foreground/9-10 has-hover:hover:ring-border focus-visible:ring-primary/8-10 aria-invalid:border-destructive/8-10 flex min-h-20 w-full rounded-md border px-3 py-2 text-sm leading-normal focus-visible:ring-1 focus-visible:outline-hidden disabled:cursor-not-allowed disabled:opacity-50 has-hover:hover:ring-1",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
