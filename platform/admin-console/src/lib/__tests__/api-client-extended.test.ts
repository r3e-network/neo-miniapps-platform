// =============================================================================
// API Client Extended Tests
// =============================================================================

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { supabaseClient, edgeClient } from "../api-client";
import { mockFetchResponse } from "./test-utils";

describe("API Client Extended", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("supabaseClient", () => {
    it("should query table without params", async () => {
      const mockData = [{ id: 1, name: "test" }];
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const result = await supabaseClient.query("users");

      expect(result).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining("/rest/v1/users"), expect.any(Object));
    });

    it("should query table with params", async () => {
      const mockData = [{ id: 1, name: "test" }];
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const result = await supabaseClient.query("users", {
        select: "*",
        order: "created_at.desc",
      });

      expect(result).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining("select="), expect.any(Object));
    });

    it("should handle query error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse({ message: "Unauthorized" }, false, 401));

      await expect(supabaseClient.query("users")).rejects.toMatchObject({
        message: expect.stringContaining("Unauthorized"),
      });
    });

    it("should query with service role", async () => {
      const mockData = [{ id: 1, name: "admin" }];
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const result = await supabaseClient.queryWithServiceRole("users");

      expect(result).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: expect.stringContaining("Bearer"),
          }),
        }),
      );
    });
  });

  describe("edgeClient", () => {
    it("should make GET request", async () => {
      const mockData = { status: "ok" };
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const result = await edgeClient.get("/health");

      expect(result).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining("/health"), expect.any(Object));
    });

    it("should make POST request", async () => {
      const mockData = { id: 1 };
      vi.spyOn(global, "fetch").mockImplementation(() => mockFetchResponse(mockData));

      const result = await edgeClient.post("/create", { name: "test" });

      expect(result).toEqual(mockData);
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining("/create"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ name: "test" }),
        }),
      );
    });

    it("should handle network error", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() => Promise.reject(new Error("Network error")));

      // The fetchJSON function re-throws errors that already have a message property
      await expect(edgeClient.get("/health")).rejects.toThrow("Network error");
    });

    it("should handle HTTP error with JSON body", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() =>
        Promise.resolve({
          ok: false,
          status: 400,
          statusText: "Bad Request",
          json: () => Promise.resolve({ message: "Invalid input" }),
        } as Response),
      );

      await expect(edgeClient.get("/invalid")).rejects.toMatchObject({
        message: "Invalid input",
      });
    });

    it("should handle HTTP error without JSON body", async () => {
      vi.spyOn(global, "fetch").mockImplementation(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          statusText: "Internal Server Error",
          json: () => Promise.reject(new Error("Not JSON")),
        } as Response),
      );

      await expect(edgeClient.get("/error")).rejects.toMatchObject({
        message: expect.stringContaining("500"),
      });
    });
  });
});
