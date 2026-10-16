import { ReactNode } from "react";

import {
  ResizableGroup,
  ResizableHandle,
  ResizablePanel,
  ResizableSizedPanel,
} from "@/components/ui/resizable";

type TProps = {
  main: ReactNode;
  aside?: ReactNode;
  storageKey?: string;
  /** The aside's width until the reader resizes it. */
  defaultAsideWidth?: number;
};

const DEFAULT_ASIDE_WIDTH = 480;
const MIN_ASIDE_WIDTH = 360;
const MAX_ASIDE_WIDTH = 900;

export function SplitView({
  main,
  aside,
  storageKey,
  defaultAsideWidth = DEFAULT_ASIDE_WIDTH,
}: TProps) {
  return (
    <ResizableGroup
      defaultSize={defaultAsideWidth}
      minSize={MIN_ASIDE_WIDTH}
      maxSize={MAX_ASIDE_WIDTH}
      storageKey={storageKey}
    >
      <ResizablePanel>{main}</ResizablePanel>
      {aside && (
        <>
          <ResizableHandle className="hidden lg:block" />
          <ResizableSizedPanel className="hidden lg:flex">{aside}</ResizableSizedPanel>
        </>
      )}
    </ResizableGroup>
  );
}
