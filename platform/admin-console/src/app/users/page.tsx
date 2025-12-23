// =============================================================================
// Users Page
// =============================================================================

"use client";

import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Spinner } from "@/components/ui/Spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/Table";
import { useUsers, useSearchUsers } from "@/lib/hooks/useUsers";
import { formatDate, truncate } from "@/lib/utils";

export default function UsersPage() {
  const [searchTerm, setSearchTerm] = useState("");
  const { data: allUsers, isLoading: allUsersLoading } = useUsers();
  const { data: searchResults, isLoading: searchLoading } = useSearchUsers(searchTerm);

  const users = searchTerm ? searchResults : allUsers;
  const isLoading = searchTerm ? searchLoading : allUsersLoading;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Users</h1>
        <p className="text-gray-600">Manage platform users</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>User Management</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <Input
              type="search"
              placeholder="Search by address or email..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>

          {isLoading ? (
            <Spinner />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User ID</TableHead>
                  <TableHead>Address</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users?.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium">{truncate(user.id, 12)}</TableCell>
                    <TableCell className="font-mono text-sm">{user.address}</TableCell>
                    <TableCell className="text-sm text-gray-500">{user.email || "N/A"}</TableCell>
                    <TableCell className="text-sm text-gray-500">{formatDate(user.created_at)}</TableCell>
                    <TableCell className="text-sm text-gray-500">{formatDate(user.updated_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          {!isLoading && users?.length === 0 && (
            <div className="py-8 text-center text-gray-500">
              {searchTerm ? "No users found matching your search" : "No users registered yet"}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
