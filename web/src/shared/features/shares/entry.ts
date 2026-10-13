// A URL fragment is never sent to the server. Consume it before rendering or
// initiating requests and remove it from browser navigation immediately.
export function consumeShareFragment(): string {
  if (window.location.pathname !== "/share") return "";
  const fragment = window.location.hash.slice(1);
  if (!fragment) return "";
  window.history.replaceState(window.history.state, "", `${window.location.pathname}${window.location.search}`);
  const tokens = new URLSearchParams(fragment).getAll("token");
  return tokens.length === 1 && /^[A-Za-z0-9_-]{43}$/.test(tokens[0]) ? tokens[0] : "";
}
