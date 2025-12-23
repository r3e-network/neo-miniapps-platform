// =============================================================================
// useAnalytics Hook Tests
// =============================================================================

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useAnalytics, useUsageByApp } from "../hooks/useAnalytics";
import { createWrapper, mockFetchResponse } from "./test-utils";

describe("useAnalytics Hooks", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("useAnalytics", () => {
    it("should fetch analytics data successfully", async () => {
      const mockData = {
        totalUsers: 100,
        activeMiniApps: 5,
        totalGasUsed: "1000.00",
        transactionsToday: 50,
      };

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useAnalytics(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith("/api/analytics");
    });

    it("should handle analytics fetch error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(null, false, 500));

      const { result } = renderHook(() => useAnalytics(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(result.current.error).toBeDefined();
    });
  });

  describe("useUsageByApp", () => {
    it("should fetch usage by app successfully", async () => {
      const mockData = [
        { appId: "app1", totalUsage: 100 },
        { appId: "app2", totalUsage: 200 },
      ];

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useUsageByApp(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith("/api/analytics/by-app");
    });

    it("should handle usage by app fetch error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(null, false, 500));

      const { result } = renderHook(() => useUsageByApp(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });
});
