// =============================================================================
// MiniApps Page
// =============================================================================

"use client";

import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Spinner } from "@/components/ui/Spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/Table";
import { useMiniApps } from "@/lib/hooks/useMiniApps";
import { formatDate, truncate } from "@/lib/utils";

export default function MiniAppsPage() {
  const { data: miniapps, isLoading, error } = useMiniApps();
  const [selectedApp, setSelectedApp] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">MiniApps</h1>
          <p className="text-gray-600">Manage registered MiniApps</p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Registered MiniApps</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Spinner />
          ) : error ? (
            <div className="text-center text-danger-600">Failed to load MiniApps</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>App ID</TableHead>
                  <TableHead>Entry URL</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Developer</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {miniapps?.map((app) => (
                  <TableRow key={app.app_id}>
                    <TableCell className="font-medium">{app.app_id}</TableCell>
                    <TableCell className="text-sm text-gray-500">{truncate(app.entry_url, 40)}</TableCell>
                    <TableCell>
                      <Badge variant={app.status === "active" ? "success" : "default"}>{app.status}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-gray-500">{truncate(app.developer_pubkey, 12)}</TableCell>
                    <TableCell className="text-sm text-gray-500">{formatDate(app.created_at)}</TableCell>
                    <TableCell>
                      <Button size="sm" variant="ghost" onClick={() => setSelectedApp(app.app_id)}>
                        View
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* MiniApp Test Harness */}
      {selectedApp && (
        <Card>
          <CardHeader>
            <CardTitle>MiniApp Test Harness: {selectedApp}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
              <p className="text-sm text-gray-600">
                Test harness for loading MiniApp in iframe (implementation pending)
              </p>
              <Button className="mt-4" variant="secondary" onClick={() => setSelectedApp(null)}>
                Close
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
