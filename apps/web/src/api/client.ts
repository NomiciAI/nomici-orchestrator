import type { ApiEnvelope, ApiErrorEnvelope } from "./types";

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
  gatewayToken = "",
  onWarnings?: (warnings: string[]) => void,
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (init.body && !(init.body instanceof FormData)) {
    headers["Content-Type"] = "application/json";
  }
  if (gatewayToken.trim() !== "") {
    headers.Authorization = `Bearer ${gatewayToken.trim()}`;
  }
  const response = await fetch(path, {
    ...init,
    headers: { ...headers, ...init.headers },
  });
  if (response.status === 401) {
    throw new Error("Gateway token did not match this Gateway");
  }
  const payload = (await response.json()) as ApiEnvelope<T> & ApiErrorEnvelope;
  if (!response.ok) {
    throw new Error(
      payload.error?.message ?? `Gateway API returned ${response.status}`,
    );
  }
  onWarnings?.(payload.warnings ?? []);
  return payload.data;
}

export function authHeaders(gatewayToken: string): Record<string, string> {
  const headers: Record<string, string> = {};
  if (gatewayToken.trim() !== "") {
    headers.Authorization = `Bearer ${gatewayToken.trim()}`;
  }
  return headers;
}
