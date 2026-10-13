import { afterEach, describe, expect, it, vi } from "vitest";
import { HttpError, requestJson } from "./http";

afterEach(() => vi.unstubAllGlobals());

describe("HTTP API error handling", () => {
  it("preserves unauthorized status for callers to request login", async () => {
    vi.stubGlobal("fetch", async () => new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 }));
    await expect(requestJson("/master/api/v1/hosts")).rejects.toMatchObject({
      name: "HttpError", status: 401, message: "unauthorized",
    });
  });

  it("does not expose an HTML proxy failure as an error message", async () => {
    vi.stubGlobal("fetch", async () => new Response("<html>private upstream details</html>", { status: 502 }));
    await expect(requestJson("/master/api/v1/hosts")).rejects.toEqual(new HttpError(502, "请求失败"));
  });

  it("accepts a successful empty response for operations with no body", async () => {
    vi.stubGlobal("fetch", async () => new Response(null, { status: 204 }));
    await expect(requestJson("/master/api/v1/auth/logout")).resolves.toBeNull();
  });
});
