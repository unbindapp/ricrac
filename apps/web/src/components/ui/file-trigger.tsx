import * as React from "react";

type TProps = {
  accept?: string;
  multiple?: boolean;
  onSelect: (files: File[]) => void;
  children: (open: () => void) => React.ReactNode;
};

function FileTrigger({ accept, multiple, onSelect, children }: TProps) {
  const inputRef = React.useRef<HTMLInputElement>(null);
  return (
    <>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={multiple}
        tabIndex={-1}
        aria-hidden
        className="hidden"
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length > 0) onSelect(files);
        }}
      />
      {children(() => inputRef.current?.click())}
    </>
  );
}

export { FileTrigger };

/** A hidden file input to open from code; render `input` once. */
function useFilePicker({ accept, multiple, onSelect }: Omit<TProps, "children">) {
  const inputRef = React.useRef<HTMLInputElement>(null);
  const input = (
    <input
      ref={inputRef}
      type="file"
      accept={accept}
      multiple={multiple}
      tabIndex={-1}
      aria-hidden
      className="hidden"
      onChange={(event) => {
        const files = Array.from(event.target.files ?? []);
        event.target.value = "";
        if (files.length > 0) onSelect(files);
      }}
    />
  );
  return { input, open: () => inputRef.current?.click() };
}

export { useFilePicker };
