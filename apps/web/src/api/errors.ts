/**
 * Extracts a user-readable error message from an Axios error response.
 * The API returns `{ "error": "..." }` on 4xx/5xx responses.
 */
export function extractApiError(
  err: unknown,
  fallback = "An unexpected error occurred.",
): string {
  const apiErr = err as {
    response?: { data?: { error?: string }; status?: number };
    message?: string;
  };
  return apiErr?.response?.data?.error ?? apiErr?.message ?? fallback;
}
