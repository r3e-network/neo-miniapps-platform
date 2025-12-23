// =============================================================================
// Tests for Utility Functions
// =============================================================================

import { describe, it, expect } from "vitest";
import { cn, formatDate, formatRelativeTime, formatNumber, formatBytes, truncate, getStatusColor } from "@/lib/utils";

describe("Utility Functions", () => {
  describe("cn", () => {
    it("should merge class names correctly", () => {
      expect(cn("text-red-500", "bg-blue-500")).toBe("text-red-500 bg-blue-500");
    });

    it("should handle conditional classes", () => {
      expect(cn("base", true && "active", false && "inactive")).toBe("base active");
    });

    it("should merge conflicting Tailwind classes", () => {
      expect(cn("p-4", "p-8")).toBe("p-8");
    });
  });

  describe("formatDate", () => {
    it("should format date string correctly", () => {
      const date = "2024-01-15T10:30:00Z";
      const formatted = formatDate(date);
      expect(formatted).toMatch(/Jan 15, 2024/);
    });

    it("should format Date object correctly", () => {
      const date = new Date("2024-01-15T10:30:00Z");
      const formatted = formatDate(date);
      expect(formatted).toMatch(/Jan 15, 2024/);
    });
  });

  describe("formatRelativeTime", () => {
    it("should return 'just now' for recent dates", () => {
      const now = new Date();
      expect(formatRelativeTime(now)).toBe("just now");
    });

    it("should return minutes ago", () => {
      const date = new Date(Date.now() - 5 * 60 * 1000);
      expect(formatRelativeTime(date)).toBe("5 minutes ago");
    });

    it("should return hours ago", () => {
      const date = new Date(Date.now() - 3 * 60 * 60 * 1000);
      expect(formatRelativeTime(date)).toBe("3 hours ago");
    });

    it("should return days ago", () => {
      const date = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000);
      expect(formatRelativeTime(date)).toBe("2 days ago");
    });
  });

  describe("formatNumber", () => {
    it("should format numbers with commas", () => {
      expect(formatNumber(1000)).toBe("1,000");
      expect(formatNumber(1000000)).toBe("1,000,000");
    });

    it("should handle zero", () => {
      expect(formatNumber(0)).toBe("0");
    });
  });

  describe("formatBytes", () => {
    it("should format bytes correctly", () => {
      expect(formatBytes(0)).toBe("0 Bytes");
      expect(formatBytes(1024)).toBe("1 KB");
      expect(formatBytes(1048576)).toBe("1 MB");
      expect(formatBytes(1073741824)).toBe("1 GB");
    });
  });

  describe("truncate", () => {
    it("should truncate long strings", () => {
      expect(truncate("Hello World", 5)).toBe("Hello...");
    });

    it("should not truncate short strings", () => {
      expect(truncate("Hello", 10)).toBe("Hello");
    });
  });

  describe("getStatusColor", () => {
    it("should return correct color for healthy status", () => {
      expect(getStatusColor("healthy")).toBe("text-success-600 bg-success-50");
    });

    it("should return correct color for unhealthy status", () => {
      expect(getStatusColor("unhealthy")).toBe("text-danger-600 bg-danger-50");
    });

    it("should return correct color for unknown status", () => {
      expect(getStatusColor("unknown")).toBe("text-gray-600 bg-gray-50");
    });

    it("should return correct color for active status", () => {
      expect(getStatusColor("active")).toBe("text-success-600 bg-success-50");
    });

    it("should return correct color for disabled status", () => {
      expect(getStatusColor("disabled")).toBe("text-danger-600 bg-danger-50");
    });
  });
});
