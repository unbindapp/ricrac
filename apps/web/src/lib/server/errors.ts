import { ApiError } from "@/lib/server/client";

export function isApiError(error: unknown, status?: number): error is ApiError {
  if (!(error instanceof ApiError)) return false;
  return status === undefined || error.status === status;
}

/** The server answers 404, or 400 when a name or slug matches nothing: either way it isn't there. */
export function isNotFoundError(error: unknown) {
  return isApiError(error) && (error.status === 404 || error.status === 400);
}

export function errorText(error: unknown, fallback = "Something went wrong") {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
