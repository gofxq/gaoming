import { requestJson } from "../../lib/http";

export async function writeAuthorizedJson<T>(apiBaseUrl: string, path: string, body: unknown, method = "POST"): Promise<T> {
  const csrf = await requestJson<{ token: string }>(`${apiBaseUrl}/auth/csrf`, {
    credentials: "include", cache: "no-store",
  });
  return requestJson<T>(`${apiBaseUrl}${path}`, {
    method, credentials: "include",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf.token },
    body: JSON.stringify(body),
  });
}
