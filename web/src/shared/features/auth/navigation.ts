// Return navigation belongs to the authenticated tenant. Reject ambiguous URL
// encodings before the router/browser can interpret them differently.
export function safeReturnTo(value: string | null, tenantCode: string): string {
  const fallback = `/${encodeURIComponent(tenantCode)}`;
  if (!value || !value.startsWith("/") || value.startsWith("//")) return fallback;
  try {
    const decoded = decodeURIComponent(value);
    if (/[\\\u0000-\u001f\u007f]/.test(decoded) || decoded.includes("%")) return fallback;
    const url = new URL(decoded, "https://gaoming.invalid");
    if (url.origin !== "https://gaoming.invalid" ||
      (url.pathname !== fallback && !url.pathname.startsWith(`${fallback}/`)) ||
      url.pathname === `${fallback}/login` || url.pathname === `${fallback}/setup`) return fallback;
    return value;
  } catch {
    return fallback;
  }
}
