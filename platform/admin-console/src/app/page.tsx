// =============================================================================
// Dashboard Home Page
// =============================================================================

"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { useServicesHealth } from "@/lib/hooks/useServices";
import { useMiniApps } from "@/lib/hooks/useMiniApps";
import { useUsers } from "@/lib/hooks/useUsers";
import { formatRelativeTime } from "@/lib/utils";

export default function DashboardPage() {
  const { data: services, isLoading: servicesLoading } = useServicesHealth();
  const { data: miniapps, isLoading: miniappsLoading } = useMiniApps();
  const { data: users, isLoading: usersLoading } = useUsers();

  const healthyServices = services?.filter((s) => s.status === "healthy").length || 0;
  const totalServices = services?.length || 0;
  const activeMiniApps = miniapps?.filter((m) => m.status === "active").length || 0;
  const totalUsers = users?.length || 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-600">Overview of your MiniApp platform</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Services Health</div>
            <div className="mt-2 flex items-baseline">
              <div className="text-3xl font-semibold text-gray-900">
                {servicesLoading ? "..." : `${healthyServices}/${totalServices}`}
              </div>
            </div>
            <Badge variant={healthyServices === totalServices ? "success" : "warning"} className="mt-2">
              {healthyServices === totalServices ? "All Healthy" : "Issues Detected"}
            </Badge>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Active MiniApps</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">{miniappsLoading ? "..." : activeMiniApps}</div>
            <p className="mt-2 text-sm text-gray-500">Total: {miniapps?.length || 0}</p>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Total Users</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">{usersLoading ? "..." : totalUsers}</div>
            <p className="mt-2 text-sm text-gray-500">Registered users</p>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Platform Status</div>
            <div className="mt-2 text-3xl font-semibold text-success-600">Online</div>
            <p className="mt-2 text-sm text-gray-500">All systems operational</p>
          </CardContent>
        </Card>
      </div>

      {/* Service Health Grid */}
      <Card>
        <CardHeader>
          <CardTitle>Service Health</CardTitle>
        </CardHeader>
        <CardContent>
          {servicesLoading ? (
            <Spinner />
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {services?.map((service) => (
                <div
                  key={service.name}
                  className="flex items-center justify-between rounded-lg border border-gray-200 p-4"
                >
                  <div>
                    <div className="font-medium text-gray-900">{service.name}</div>
                    <div className="text-sm text-gray-500">{formatRelativeTime(service.lastCheck)}</div>
                  </div>
                  <Badge
                    variant={
                      service.status === "healthy" ? "success" : service.status === "unhealthy" ? "danger" : "default"
                    }
                  >
                    {service.status}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Recent MiniApps */}
      <Card>
        <CardHeader>
          <CardTitle>Recent MiniApps</CardTitle>
        </CardHeader>
        <CardContent>
          {miniappsLoading ? (
            <Spinner />
          ) : (
            <div className="space-y-3">
              {miniapps?.slice(0, 5).map((app) => (
                <div key={app.app_id} className="flex items-center justify-between border-b border-gray-100 pb-3">
                  <div>
                    <div className="font-medium text-gray-900">{app.app_id}</div>
                    <div className="text-sm text-gray-500">{formatRelativeTime(app.created_at)}</div>
                  </div>
                  <Badge variant={app.status === "active" ? "success" : "default"}>{app.status}</Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
