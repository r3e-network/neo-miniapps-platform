// =============================================================================
// Header Component
// =============================================================================

"use client";

export function Header() {
  return (
    <header className="sticky top-0 z-10 border-b border-gray-200 bg-white">
      <div className="flex h-16 items-center justify-between px-6">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Admin Dashboard</h2>
          <p className="text-sm text-gray-500">Monitor and manage your MiniApp platform</p>
        </div>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-600">Local Development</span>
        </div>
      </div>
    </header>
  );
}
