import { MaterialShapes, Morph, morphToPath, type RoundedPolygon } from "material-shapes-ts";
import { useEffect, useRef } from "react";

const MORPH_INTERVAL_MS = 650;
const GLOBAL_ROTATION_MS = 4666;
const ACTIVE_INDICATOR_SCALE = 38 / 48;
const MORPH_DAMPING = 0.6;
const MORPH_STIFFNESS = 200;

let morphs: Morph[] | null = null;
let shapesScale = 1;

function morphSequence() {
  if (morphs) return morphs;
  const shapes = [
    MaterialShapes.SoftBurst,
    MaterialShapes.Cookie9Sided,
    MaterialShapes.Pentagon,
    MaterialShapes.Pill,
    MaterialShapes.Sunny,
    MaterialShapes.Cookie4Sided,
    MaterialShapes.Oval,
  ];
  shapesScale = Math.min(...shapes.map(rotationSafeScale)) * ACTIVE_INDICATOR_SCALE;
  morphs = shapes.map((shape, index) => new Morph(shape, shapes[(index + 1) % shapes.length]));
  return morphs;
}

/** How far a shape shrinks so it never clips while it turns. */
function rotationSafeScale(shape: RoundedPolygon) {
  const [left, top, right, bottom] = shape.calculateBounds();
  const [maxLeft, maxTop, maxRight, maxBottom] = shape.calculateMaxBounds();
  return Math.max((right - left) / (maxRight - maxLeft), (bottom - top) / (maxBottom - maxTop));
}

function springAt(seconds: number) {
  const natural = Math.sqrt(MORPH_STIFFNESS);
  const damped = natural * Math.sqrt(1 - MORPH_DAMPING * MORPH_DAMPING);
  const decay = Math.exp(-MORPH_DAMPING * natural * seconds);
  return (
    1 -
    decay *
      (Math.cos(damped * seconds) +
        ((MORPH_DAMPING * natural) / damped) * Math.sin(damped * seconds))
  );
}

/** Material 3 Expressive's loading indicator: Material shapes morphing into each other as they turn. */
export function LoadingIndicator({ label = "Loading" }: { label?: string }) {
  const pathRef = useRef<SVGPathElement>(null);

  useEffect(() => {
    const sequence = morphSequence();
    const path = pathRef.current;
    if (!path) return;
    const draw = (elapsed: number) => {
      const index = Math.floor(elapsed / MORPH_INTERVAL_MS) % sequence.length;
      const progress = springAt((elapsed % MORPH_INTERVAL_MS) / 1000);
      const turn =
        progress * 90 +
        (index + 1) * 90 +
        (360 * (elapsed % GLOBAL_ROTATION_MS)) / GLOBAL_ROTATION_MS;
      path.setAttribute(
        "d",
        morphToPath(sequence[index], Math.min(1, Math.max(0, progress))).toSvgPathData(),
      );
      path.setAttribute(
        "transform",
        `rotate(${turn % 360} 0.5 0.5) translate(0.5 0.5) scale(${shapesScale}) translate(-0.5 -0.5)`,
      );
    };
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
      draw(0);
      return;
    }
    const start = performance.now();
    let frame = requestAnimationFrame(function tick(now) {
      draw(Math.max(0, now - start));
      frame = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <svg
      role="progressbar"
      aria-label={label}
      viewBox="0 0 1 1"
      className="text-m3-primary size-12"
    >
      <path ref={pathRef} className="fill-current" />
    </svg>
  );
}
