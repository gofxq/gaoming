import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { AppConfigProvider } from "../config/AppConfigProvider";
import { AuthProvider, useAuth } from "./AuthProvider";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("logout cancels private requests and removes cached member data", async () => {
  vi.stubGlobal("fetch", async (input: string) => new Response(JSON.stringify(input.endsWith("/csrf") ? { token: "csrf" } : input.endsWith("/logout") ? null : {
    authenticated: true, user: { id: 1, tenant_code: "team", role: "admin", status: "active" },
  })));
  const client = new QueryClient();
  function Session() {
    const { user, signOut } = useAuth();
    return <><span>{user ? "authenticated" : "signed out"}</span><button onClick={() => void signOut()}>Logout</button></>;
  }
  render(<QueryClientProvider client={client}><AppConfigProvider><AuthProvider><Session /></AuthProvider></AppConfigProvider></QueryClientProvider>);
  await screen.findByText("authenticated");
  const key = ["private", "admin-users", "team", 1];
  client.setQueryData(key, [{ display_name: "previous-private-member" }]);
  let finish!: (value: unknown) => void;
  const pending = client.fetchQuery({ queryKey: ["private", "pending"], queryFn: () => new Promise((resolve) => { finish = resolve; }) }).catch(() => undefined);
  fireEvent.click(screen.getByText("Logout"));
  await screen.findByText("signed out");
  expect(client.getQueryData(key)).toBeUndefined();
  await act(async () => { finish([{ display_name: "late-private-member" }]); await pending; });
  await waitFor(() => expect(client.getQueryData(["private", "pending"])).toBeUndefined());
});
