import { describe, expect, it } from "vitest";
import { safeReturnTo } from "./navigation";

describe("login return navigation", () => {
  it.each(["//evil.example", "https://evil.example", "/\\evil.example", "/other/users", "/default-other", "/default/../other", "/default/%2e%2e/other", "/default/%5cevil", "/default/login", "/default/setup"])("rejects external, ambiguous, cross-tenant or recursive return path %s", (value) => {
    expect(safeReturnTo(value, "default")).toBe("/default");
  });

  it("preserves a same-tenant path with filters and detail context", () => {
    expect(safeReturnTo("/default/hosts/host-1?status=offline#cpu", "default")).toBe("/default/hosts/host-1?status=offline#cpu");
  });
});
