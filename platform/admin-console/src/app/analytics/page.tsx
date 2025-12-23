// =============================================================================
// Analytics Page
// =============================================================================

"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Spinner } from "@/components/ui/Spinner";
import { useAnalytics, useMiniAppUsage } from "@/lib/hooks/useAnalytics";
import { formatNumber } from "@/lib/utils";

export default function AnalyticsPage() {
  const { data: analytics, isLoading: analyticsLoading } = useAnalytics();
  const { data: usage, isLoading: usageLoading } = useMiniAppUsage(30);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Analytics</h1>
        <p className="text-gray-600">Platform usage and metrics</p>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Total Users</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">
              {analyticsLoading ? "..." : formatNumber(analytics?.totalUsers || 0)}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Total MiniApps</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">
              {analyticsLoading ? "..." : formatNumber(analytics?.totalMiniApps || 0)}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">Total Transactions</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">
              {analyticsLoading ? "..." : formatNumber(analytics?.totalTransactions || 0)}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="text-sm font-medium text-gray-600">GAS Used Today</div>
            <div className="mt-2 text-3xl font-semibold text-gray-900">
              {analyticsLoading ? "..." : formatNumber(analytics?.gasUsageToday || 0)}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Usage Chart Placeholder */}
      <Card>
        <CardHeader>
          <CardTitle>Usage Over Time (Last 30 Days)</CardTitle>
        </CardHeader>
        <CardContent>
          {usageLoading ? (
            <Spinner />
          ) : (
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-8 text-center">
              <p className="text-gray-600">
                Chart visualization placeholder - {usage?.length || 0} data points available
              </p>
              <p className="mt-2 text-sm text-gray-500">Integration with charting library (recharts) pending</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Usage by App */}
      <Card>
        <CardHeader>
          <CardTitle>Usage by MiniApp</CardTitle>
        </CardHeader>
        <CardContent>
          {analyticsLoading ? (
            <Spinner />
          ) : (
            <div className="space-y-3">
              {analytics?.usageByApp?.slice(0, 10).map((app) => (
                <div key={app.app_id} className="flex items-center justify-between border-b border-gray-100 pb-3">
                  <div>
                    <div className="font-medium text-gray-900">{app.app_id}</div>
                    <div className="text-sm text-gray-500">{app.user_count} users</div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-medium text-gray-900">GAS: {formatNumber(app.total_gas)}</div>
                    <div className="text-sm text-gray-500">GOV: {formatNumber(app.total_governance)}</div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
