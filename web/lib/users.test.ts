// @vitest-environment jsdom
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";

const authedGet = vi.fn();
vi.mock("./api", () => ({ authedGet: (path: string) => authedGet(path) }));

import { useUserSearch, type SearchUser } from "./users";

const alice: SearchUser = { id: "u-alice", display_name: "Alice", online: true };

beforeEach(() => {
  vi.useFakeTimers();
  authedGet.mockReset();
  authedGet.mockResolvedValue([alice]);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useUserSearch", () => {
  it("does not query for fewer than 2 characters", () => {
    const { result } = renderHook(() => useUserSearch());
    act(() => result.current.setQuery("a"));
    act(() => vi.advanceTimersByTime(500));
    expect(authedGet).not.toHaveBeenCalled();
    expect(result.current.results).toEqual([]);
  });

  it("debounces then queries /users/search and maps results", async () => {
    const { result } = renderHook(() => useUserSearch());
    act(() => result.current.setQuery("ali"));
    // Before the debounce window elapses, no request has gone out.
    act(() => vi.advanceTimersByTime(100));
    expect(authedGet).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    expect(authedGet).toHaveBeenCalledWith("/users/search?q=ali");
    expect(result.current.results).toEqual([alice]);
  });

  it("only keeps the latest query's results when typing quickly", async () => {
    const { result } = renderHook(() => useUserSearch());
    act(() => result.current.setQuery("al"));
    act(() => vi.advanceTimersByTime(100)); // first timer not yet fired
    act(() => result.current.setQuery("alice")); // supersedes "al"
    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    expect(authedGet).toHaveBeenCalledTimes(1);
    expect(authedGet).toHaveBeenCalledWith("/users/search?q=alice");
  });
});
