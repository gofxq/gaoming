import { afterEach, expect, it } from "vitest";
import { consumeShareFragment } from "./entry";

afterEach(() => window.history.replaceState(null, "", "/"));

it("removes a share credential from navigation before returning it", () => {
  const token = "A".repeat(43);
  window.history.replaceState(null, "", `/share#token=${token}`);
  expect(consumeShareFragment()).toBe(token);
  expect(window.location.hash).toBe("");
  expect(window.location.pathname).toBe("/share");
});

it("does not consume private detail anchors and rejects malformed or ambiguous credentials", () => {
  window.history.replaceState(null, "", "/team/hosts/host-1#cpu");
  expect(consumeShareFragment()).toBe("");
  expect(window.location.hash).toBe("#cpu");
  for (const fragment of ["#token=bad", "#token=" + "A".repeat(43) + "&token=" + "B".repeat(43)]) {
    window.history.replaceState(null, "", `/share${fragment}`);
    expect(consumeShareFragment()).toBe("");
    expect(window.location.hash).toBe("");
  }
});
