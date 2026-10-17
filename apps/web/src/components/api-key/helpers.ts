import { z } from "zod";

import type { TScope } from "@/lib/queries/api-keys";

export type TAccess = "all" | "specific";

export const accessItems: { value: TAccess; label: string }[] = [
  { value: "all", label: "All my workspaces, including ones I join later" },
  { value: "specific", label: "Only the workspaces I pick" },
];

export type TScopeLevel = "read" | "write" | "admin";

export const scopeLevelItems: { value: TScopeLevel; label: string }[] = [
  { value: "read", label: "Read" },
  { value: "write", label: "Read and write" },
  { value: "admin", label: "Read, write and admin" },
];

export const scopeLevelDescriptions: Record<TScopeLevel, string> = {
  read: "Reads everything you can see.",
  write: "Also creates and changes issues, projects, comments and more.",
  admin: "Also manages members, teams and settings where you're an admin.",
};

export function scopesOf(level: TScopeLevel): TScope[] {
  if (level === "read") return ["read"];
  if (level === "write") return ["read", "write"];
  return ["read", "write", "admin"];
}

/** The widest of the scopes a key or grant holds. */
export function scopeLevel(scopes: string[]): TScopeLevel {
  if (scopes.includes("admin")) return "admin";
  if (scopes.includes("write")) return "write";
  return "read";
}

export const accessFormShape = {
  access: z.enum(["all", "specific"]),
  workspaceIds: z.array(z.string()),
};

export function hasPickedWorkspace(value: { access: TAccess; workspaceIds: string[] }) {
  return value.access === "all" || value.workspaceIds.length > 0;
}

export const pickWorkspaceMessage = {
  message: "Pick at least one workspace.",
  path: ["workspaceIds"],
};

/** The workspaces part of a key or connected app, from the names the server lists. */
export function describeAccess(access: { allWorkspaces: boolean; workspaces: string[] }) {
  if (access.allWorkspaces) return "All workspaces";
  if (access.workspaces.length === 0) return "No workspaces";
  return access.workspaces.join(", ");
}
