// =============================================================================
// Services Health Page
// =============================================================================

"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/Table";
import { useServicesHealth } from "@/lib/hooks/useServices";
import { formatRelativeTime } from "@/lib/utils";

export default function ServicesPage() {
  const { data: services, isLoading, error } = useServicesHealth(30000);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Services</h1>
        <p className="text-gray-600">Monitor service health and status</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Service Health Status</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Spinner />
          ) : error ? (
            <div className="text-center text-danger-600">Failed to load services</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Service Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>URL</TableHead>
                  <TableHead>Version</TableHead>
                  <TableHead>Last Check</TableHead>
                  <TableHead>Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {services?.map((service) => (
                  <TableRow key={service.name}>
                    <TableCell className="font-medium">{service.name}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          service.status === "healthy"
                            ? "success"
                            : service.status === "unhealthy"
                              ? "danger"
                              : "default"
                        }
                      >
                        {service.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-gray-500">{service.url}</TableCell>
                    <TableCell>{service.version || "N/A"}</TableCell>
                    <TableCell className="text-sm text-gray-500">{formatRelativeTime(service.lastCheck)}</TableCell>
                    <TableCell className="text-sm text-danger-600">{service.error || "-"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
