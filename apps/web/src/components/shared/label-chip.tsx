type TProps = {
  name: string;
  color?: string | null;
};

export function LabelChip({ name, color }: TProps) {
  return (
    <span className="bg-background text-foreground dux:h-8 dux:gap-2 dux:rounded-sm dux:border-m3-outline-variant dux:bg-transparent dux:pr-3 dux:pl-2 dux:type-label-large dux:text-m3-on-surface-variant inline-flex h-5 max-w-40 shrink-0 items-center gap-1.5 rounded-full border px-2 text-xs leading-none">
      <span
        aria-hidden
        className="bg-muted-foreground dux:mx-1 dux:size-2.5 size-2 shrink-0 rounded-full"
        style={color ? { backgroundColor: color } : undefined}
      />
      <span className="truncate">{name}</span>
    </span>
  );
}
