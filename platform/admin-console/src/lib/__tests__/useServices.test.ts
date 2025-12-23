// =============================================================================
// useServices Hook Tests
// =============================================================================

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useServicesHealth, useServiceHealth } from "../hooks/useServices";
import { createWrapper, mockFetchResponse } from "./test-utils";

describe("useServices Hooks", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("useServicesHealth", () => {
    it("should fetch services health successfully", async () => {
      const mockData = [
        { name: "neoaccounts", status: "healthy", version: "1.0.0" },
        { name: "gasbank", status: "healthy", version: "1.0.0" },
      ];

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useServicesHealth(0), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith("/api/services/health");
    });

    it("should handle fetch error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(null, false, 500));

      const { result } = renderHook(() => useServicesHealth(0), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(result.current.error).toBeDefined();
    });

    it("should use default refetch interval", () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse([]));

      const { result } = renderHook(() => useServicesHealth(), {
        wrapper: createWrapper(),
      });

      // Hook should be defined with default interval
      expect(result.current).toBeDefined();
    });
  });

  describe("useServiceHealth", () => {
    it("should fetch single service health", async () => {
      const mockData = { name: "neoaccounts", status: "healthy" };

      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const { result } = renderHook(() => useServiceHealth("neoaccounts"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith("/api/services/health?service=neoaccounts");
    });

    it("should handle service not found", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse({ error: "Not found" }, false, 404));

      const { result } = renderHook(() => useServiceHealth("unknown"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });
});
