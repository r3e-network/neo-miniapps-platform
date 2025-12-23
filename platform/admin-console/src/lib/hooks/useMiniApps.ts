// =============================================================================
// React Query Hooks - MiniApps
// =============================================================================

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { supabaseClient } from "@/lib/api-client";
import type { MiniApp } from "@/types";

/**
 * Fetch all MiniApps
 */
async function fetchMiniApps(): Promise<MiniApp[]> {
  return supabaseClient.query<MiniApp[]>("miniapps", {
    select: "*",
    order: "created_at.desc",
  });
}

/**
 * Fetch single MiniApp by ID
 */
async function fetchMiniApp(appId: string): Promise<MiniApp> {
  const result = await supabaseClient.query<MiniApp[]>("miniapps", {
    select: "*",
    app_id: `eq.${appId}`,
  });
  if (!result || result.length === 0) {
    throw new Error(`MiniApp ${appId} not found`);
  }
  return result[0];
}

/**
 * Hook to fetch all MiniApps
 */
export function useMiniApps() {
  return useQuery({
    queryKey: ["miniapps"],
    queryFn: fetchMiniApps,
    staleTime: 60000,
  });
}

/**
 * Hook to fetch single MiniApp
 */
export function useMiniApp(appId: string) {
  return useQuery({
    queryKey: ["miniapps", appId],
    queryFn: () => fetchMiniApp(appId),
    enabled: !!appId,
    staleTime: 60000,
  });
}

/**
 * Hook to update MiniApp status
 */
export function useUpdateMiniAppStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ appId, status }: { appId: string; status: "active" | "disabled" }) => {
      const response = await fetch("/api/miniapps/update-status", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ appId, status }),
      });

      if (!response.ok) {
        throw new Error("Failed to update MiniApp status");
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["miniapps"] });
    },
  });
}
