// =============================================================================
// React Query Hooks - Users
// =============================================================================

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { supabaseClient } from "@/lib/api-client";
import type { User } from "@/types";

/**
 * Fetch all users
 */
async function fetchUsers(): Promise<User[]> {
  return supabaseClient.query<User[]>("users", {
    select: "*",
    order: "created_at.desc",
  });
}

/**
 * Fetch single user by ID
 */
async function fetchUser(userId: string): Promise<User> {
  const result = await supabaseClient.query<User[]>("users", {
    select: "*",
    id: `eq.${userId}`,
  });
  if (!result || result.length === 0) {
    throw new Error(`User ${userId} not found`);
  }
  return result[0];
}

/**
 * Hook to fetch all users
 */
export function useUsers() {
  return useQuery({
    queryKey: ["users"],
    queryFn: fetchUsers,
    staleTime: 60000,
  });
}

/**
 * Hook to fetch single user
 */
export function useUser(userId: string) {
  return useQuery({
    queryKey: ["users", userId],
    queryFn: () => fetchUser(userId),
    enabled: !!userId,
    staleTime: 60000,
  });
}

/**
 * Hook to search users
 */
export function useSearchUsers(searchTerm: string) {
  return useQuery({
    queryKey: ["users", "search", searchTerm],
    queryFn: async () => {
      if (!searchTerm) return [];
      return supabaseClient.query<User[]>("users", {
        select: "*",
        or: `address.ilike.%${searchTerm}%,email.ilike.%${searchTerm}%`,
      });
    },
    enabled: searchTerm.length > 0,
    staleTime: 30000,
  });
}
