import type { TeamDetail } from "@/api/ops.gen";

export type TEstimateType = TeamDetail["estimateType"];

const baseScales: Record<Exclude<TEstimateType, "notUsed">, number[]> = {
  exponential: [1, 2, 4, 8, 16],
  fibonacci: [1, 2, 3, 5, 8],
  linear: [1, 2, 3, 4, 5],
  tShirt: [1, 2, 3, 5, 8],
};

const extendedScales: Record<Exclude<TEstimateType, "notUsed">, number[]> = {
  exponential: [32, 64],
  fibonacci: [13, 21],
  linear: [6, 7, 8, 9, 10],
  tShirt: [13, 21],
};

const tshirtLabels: Record<number, string> = {
  0: "None",
  1: "XS",
  2: "S",
  3: "M",
  5: "L",
  8: "XL",
  13: "XXL",
  21: "XXXL",
};

export const estimateTypeLabels: Record<TEstimateType, string> = {
  notUsed: "Not in use",
  exponential: "Exponential (1, 2, 4, 8, 16)",
  fibonacci: "Fibonacci (1, 2, 3, 5, 8)",
  linear: "Linear (1, 2, 3, 4, 5)",
  tShirt: "T-shirt sizes (XS, S, M, L, XL)",
};

export function estimateOptions({
  type,
  allowZero,
  extended,
}: {
  type: TEstimateType;
  allowZero: boolean;
  extended: boolean;
}) {
  if (type === "notUsed") return [];
  const values = [...baseScales[type], ...(extended ? extendedScales[type] : [])];
  return allowZero ? [0, ...values] : values;
}

export function formatEstimate(value: number, type: TEstimateType) {
  if (type === "tShirt") return tshirtLabels[value] ?? String(value);
  return String(value);
}
