import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";

import { isDomain, parseDomains, parseWholeNumber } from "@/lib/helpers/customers";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TCustomerFilters = {
  query?: string;
  status?: string;
  tier?: string;
  includeArchived?: boolean;
};

/** Whose needs to list: a customer's, an issue's or a project's. */
export type TNeedScope = { customer: string } | { issue: string } | { project: string };

export const queryKeyCustomers = {
  all: (workspaceId: string) => [workspaceId, "customers"] as const,
  list: (workspaceId: string, filters: TCustomerFilters) =>
    [workspaceId, "customers", "list", filters] as const,
  detail: (workspaceId: string, id: string) => [workspaceId, "customers", "detail", id] as const,
  statuses: (workspaceId: string) => [workspaceId, "customers", "statuses"] as const,
  tiers: (workspaceId: string) => [workspaceId, "customers", "tiers"] as const,
  needs: (workspaceId: string, scope: TNeedScope) =>
    [workspaceId, "customers", "needs", scope] as const,
};

export const customersQuery = (workspaceId: string, filters: TCustomerFilters = {}) =>
  queryOptions({
    queryKey: queryKeyCustomers.list(workspaceId, filters),
    queryFn: () =>
      callAll(workspaceId, "list_customers", {
        query: filters.query || null,
        status: filters.status || null,
        tier: filters.tier || null,
        includeArchived: filters.includeArchived,
      }),
  });

/** A customer by id, slug, name or domain, with its needs. */
export const customerQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyCustomers.detail(workspaceId, id),
    queryFn: () => call(workspaceId, "get_customer", { id }),
  });

export const customerStatusesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyCustomers.statuses(workspaceId),
    queryFn: () => callAll(workspaceId, "list_customer_statuses", {}),
  });

export const customerTiersQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyCustomers.tiers(workspaceId),
    queryFn: () => callAll(workspaceId, "list_customer_tiers", {}),
  });

export const customerNeedsQuery = (workspaceId: string, scope: TNeedScope) =>
  queryOptions({
    queryKey: queryKeyCustomers.needs(workspaceId, scope),
    queryFn: () => callAll(workspaceId, "list_customer_needs", scope),
  });

/** Customers, their options and needs all sit under one key, so any change refreshes them all. */
function useInvalidateCustomers(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyCustomers.all(workspaceId) });
}

export function useSaveCustomerMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_customer">) => call(workspaceId, "save_customer", args),
    meta: { errorTitle: "Could not save customer" },
    onSuccess: invalidate,
  });
}

export function useDeleteCustomerMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_customer", { id }),
    meta: { errorTitle: "Could not delete customer" },
    onSuccess: invalidate,
  });
}

export function useSaveCustomerStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_customer_status">) =>
      call(workspaceId, "save_customer_status", args),
    meta: { errorTitle: "Could not save status" },
    onSuccess: invalidate,
  });
}

export function useDeleteCustomerStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_customer_status", { id }),
    meta: { errorTitle: "Could not delete status" },
    onSuccess: invalidate,
  });
}

export function useSaveCustomerTierMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_customer_tier">) =>
      call(workspaceId, "save_customer_tier", args),
    meta: { errorTitle: "Could not save tier" },
    onSuccess: invalidate,
  });
}

export function useDeleteCustomerTierMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_customer_tier", { id }),
    meta: { errorTitle: "Could not delete tier" },
    onSuccess: invalidate,
  });
}

export function useSaveCustomerNeedMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_customer_need">) =>
      call(workspaceId, "save_customer_need", args),
    meta: { errorTitle: "Could not save request" },
    onSuccess: invalidate,
  });
}

export function useDeleteCustomerNeedMutation(workspaceId: string) {
  const invalidate = useInvalidateCustomers(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_customer_need", { id }),
    meta: { errorTitle: "Could not delete request" },
    onSuccess: invalidate,
  });
}

export const customerNameMaxLength = 120;

const wholeNumber = (label: string) =>
  z
    .string()
    .trim()
    .refine((value) => parseWholeNumber(value) !== undefined, {
      message: `${label} is a whole number.`,
    });

export const CustomerFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(customerNameMaxLength, `Keep the name under ${customerNameMaxLength} characters.`),
  domains: z.string().refine((text) => parseDomains(text).every(isDomain), {
    message: "Use domains like acme.com, separated by commas.",
  }),
  revenue: wholeNumber("Revenue"),
  size: wholeNumber("Size"),
});
