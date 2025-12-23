// =============================================================================
// Tests for API Client
// =============================================================================

import { describe, it, expect, vi, beforeEach } from "vitest";
import { checkServiceHealth } from "@/lib/api-client";

describe("API Client", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("checkServiceHealth", () => {
    it("should return healthy status for successful response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ status: "healthy", version: "1.0.0" }),
      });

      const result = await checkServiceHealth("test-service", "http://test.local");

      expect(result.status).toBe("healthy");
      expect(result.data).toEqual({ status: "healthy", version: "1.0.0" });
    });

    it("should return unhealthy status for failed response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      });

      const result = await checkServiceHealth("test-service", "http://test.local");

      expect(result.status).toBe("unhealthy");
      expect(result.error).toBe("HTTP 500");
    });

    it("should return unhealthy status for network error", async () => {
      global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

      const result = await checkServiceHealth("test-service", "http://test.local");

      expect(result.status).toBe("unhealthy");
      expect(result.error).toBe("Network error");
    });
  });
});
