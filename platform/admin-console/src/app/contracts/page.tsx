// =============================================================================
// Contracts Page
// =============================================================================

"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";

const CONTRACTS = [
  { name: "AppRegistry", hash: "0x...", deployed: true, network: "TestNet" },
  { name: "PaymentHub", hash: "0x...", deployed: true, network: "TestNet" },
  { name: "Governance", hash: "0x...", deployed: true, network: "TestNet" },
  { name: "PriceFeed", hash: "0x...", deployed: true, network: "TestNet" },
  { name: "RandomnessLog", hash: "0x...", deployed: true, network: "TestNet" },
  { name: "AutomationAnchor", hash: "0x...", deployed: true, network: "TestNet" },
];

export default function ContractsPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Contracts</h1>
          <p className="text-gray-600">Manage smart contracts</p>
        </div>
        <Button>Deploy New Contract</Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Deployed Contracts</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {CONTRACTS.map((contract) => (
              <div
                key={contract.name}
                className="flex items-center justify-between rounded-lg border border-gray-200 p-4"
              >
                <div>
                  <div className="font-medium text-gray-900">{contract.name}</div>
                  <div className="text-sm text-gray-500">Hash: {contract.hash}</div>
                  <div className="text-sm text-gray-500">Network: {contract.network}</div>
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant={contract.deployed ? "success" : "default"}>
                    {contract.deployed ? "Deployed" : "Not Deployed"}
                  </Badge>
                  <Button size="sm" variant="ghost">
                    View
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Contract Deployment Wizard</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-gray-200 bg-gray-50 p-8 text-center">
            <p className="text-gray-600">Contract deployment wizard placeholder</p>
            <p className="mt-2 text-sm text-gray-500">Step-by-step deployment flow implementation pending</p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
