// =============================================================================
// useMiniApps Hook Tests
// =============================================================================

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useMiniApps, useMiniApp } from "../hooks/useMiniApps";
import { createWrapper, mockFetchResponse } from "./test-utils";

describe("useMiniApps Hooks", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("useMiniApps", () => {
    it("should fetch all miniapps successfully", async () => {
      const mockData = [
        { app_id: "app1", name: "App 1", status: "active" },
        { app_id: "app2", name: "App 2", status: "disabled" },
      ];

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useMiniApps(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData);
    });

    it("should handle fetch error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(null, false, 500));

      const { result } = renderHook(() => useMiniApps(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });

  describe("useMiniApp", () => {
    it("should fetch single miniapp successfully", async () => {
      const mockData = [{ app_id: "app1", name: "App 1", status: "active" }];

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useMiniApp("app1"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData[0]);
    });

    it("should handle miniapp not found", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse([]));

      const { result } = renderHook(() => useMiniApp("unknown"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });

    it("should not fetch when appId is empty", () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse([]));

      const { result } = renderHook(() => useMiniApp(""), {
        wrapper: createWrapper(),
      });

      expect(result.current.fetchStatus).toBe("idle");
      expect(global.fetch).not.toHaveBeenCalled();
    });
  });
});
