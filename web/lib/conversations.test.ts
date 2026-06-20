import { describe, it, expect } from "vitest";
import { convLabel } from "./conversations";

describe("convLabel", () => {
  it("labels a DM with the peer's name", () => {
    expect(convLabel({ ID: "dm_x", Kind: "dm", Name: "", PeerName: "Alice" })).toBe("Alice");
  });

  it("falls back to 'Direct message' when the peer name is missing", () => {
    expect(convLabel({ ID: "dm_x", Kind: "dm", Name: "", PeerName: "" })).toBe("Direct message");
  });

  it("prefixes a channel with '#'", () => {
    expect(convLabel({ ID: "c1", Kind: "channel", Name: "general", PeerName: "" })).toBe("# general");
  });

  it("shows a group's name as-is", () => {
    expect(convLabel({ ID: "g1", Kind: "group", Name: "Team", PeerName: "" })).toBe("Team");
  });
});
