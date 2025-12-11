#!/usr/bin/env python3
"""
Service Layer Contract Initialization Script

This script initializes all deployed Service Layer contracts:
1. Registers TEE accounts with the Gateway
2. Sets service fees
3. Registers services with the Gateway
4. Configures service contracts with Gateway address
5. Funds user accounts for testing

Usage:
    python3 initialize.py [network]

Networks:
    - neoexpress (default): Local Neo Express
    - testnet: Neo N3 TestNet
"""

import json
import os
import sys
import time
from pathlib import Path
from dataclasses import dataclass
from typing import Optional, Dict, Any

# Configuration
SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
CONFIG_DIR = PROJECT_ROOT / "deploy" / "config"
DEPLOYED_FILE = CONFIG_DIR / "deployed_contracts.json"

# Service fees in GAS (8 decimals)
SERVICE_FEES = {
    "oracle": 10000000,      # 0.1 GAS
    "vrf": 10000000,         # 0.1 GAS
    "mixer": 50000000,       # 0.5 GAS
    "datafeeds": 5000000,    # 0.05 GAS
    "automation": 20000000,  # 0.2 GAS
    "confidential": 100000000,  # 1.0 GAS
}

# Service contract mapping
SERVICE_CONTRACTS = {
    "vrf": "VRFService",
    "mixer": "NeoVaultService",
    "datafeeds": "DataFeedsService",
    "automation": "NeoFlowService",
}


@dataclass
class NetworkConfig:
    name: str
    rpc_url: str
    network_magic: int
    neo_express_config: Optional[str] = None


NETWORKS = {
    "neoexpress": NetworkConfig(
        name="neoexpress",
        rpc_url="http://127.0.0.1:50012",
        network_magic=1234512345,
        neo_express_config=str(CONFIG_DIR / "default.neo-express"),
    ),
    "testnet": NetworkConfig(
        name="testnet",
        rpc_url="https://testnet1.neo.coz.io:443",
        network_magic=877933390,
    ),
}


class ContractInitializer:
    """Initialize Service Layer contracts."""

    def __init__(self, network: str = "neoexpress"):
        self.network = NETWORKS.get(network)
        if not self.network:
            raise ValueError(f"Unknown network: {network}")

        self.deployed = self._load_deployed_contracts()
        self.tee_pubkey = self._get_tee_pubkey()

    def _load_deployed_contracts(self) -> Dict[str, str]:
        """Load deployed contract addresses."""
        if not DEPLOYED_FILE.exists():
            raise FileNotFoundError(f"Deployed contracts file not found: {DEPLOYED_FILE}")

        with open(DEPLOYED_FILE) as f:
            return json.load(f)

    def _get_tee_pubkey(self) -> str:
        """Get TEE account public key."""
        if self.network.neo_express_config:
            # For Neo Express, get from wallet
            return self._get_wallet_pubkey("tee")
        return os.environ.get("TEE_PUBKEY", "")

    def _get_wallet_pubkey(self, wallet_name: str) -> str:
        """Get public key from Neo Express wallet."""
        import subprocess
        import json

        result = subprocess.run(
            ["neoxp", "wallet", "export", wallet_name, "-i", self.network.neo_express_config],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            print(f"  Warning: Failed to export wallet {wallet_name}: {result.stderr}")
            return ""

        try:
            wallet_data = json.loads(result.stdout)
            # Neo Express exports wallet with accounts array containing public keys
            if "accounts" in wallet_data and len(wallet_data["accounts"]) > 0:
                account = wallet_data["accounts"][0]
                if "key" in account:
                    # The key field contains the public key in hex format
                    return account.get("key", "")
            return ""
        except json.JSONDecodeError:
            print(f"  Warning: Failed to parse wallet export for {wallet_name}")
            return ""

    def invoke(self, contract: str, method: str, *args) -> Dict[str, Any]:
        """Invoke a contract method."""
        contract_hash = self.deployed.get(contract)
        if not contract_hash:
            print(f"  Warning: Contract {contract} not found in deployed contracts")
            return {"error": "contract not found"}

        if self.network.neo_express_config:
            return self._invoke_neoexpress(contract_hash, method, args)
        else:
            return self._invoke_rpc(contract_hash, method, args)

    def _invoke_neoexpress(self, contract_hash: str, method: str, args: tuple) -> Dict[str, Any]:
        """Invoke using Neo Express."""
        import subprocess

        # Build args string
        args_str = " ".join(str(a) for a in args)

        cmd = [
            "neoxp", "contract", "invoke",
            contract_hash, method,
            "-i", self.network.neo_express_config,
            "--account", "owner",
        ]
        if args_str:
            cmd.extend(args_str.split())

        result = subprocess.run(cmd, capture_output=True, text=True)
        return {"stdout": result.stdout, "stderr": result.stderr, "returncode": result.returncode}

    def _invoke_rpc(self, contract_hash: str, method: str, args: tuple) -> Dict[str, Any]:
        """Invoke using JSON-RPC."""
        import requests

        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "invokefunction",
            "params": [contract_hash, method, list(args)],
        }
        response = requests.post(self.network.rpc_url, json=payload)
        return response.json()

    def initialize_gateway(self):
        """Initialize the ServiceLayerGateway contract."""
        print("\n=== Initializing ServiceLayerGateway ===")

        gateway_hash = self.deployed.get("ServiceLayerGateway")
        if not gateway_hash:
            print("  Error: ServiceLayerGateway not deployed")
            return

        # 1. Register TEE account
        print("  Registering TEE account...")
        tee_address = self._get_wallet_address("tee")
        self.invoke("ServiceLayerGateway", "registerTEEAccount", tee_address, self.tee_pubkey)

        # 2. Set service fees
        print("  Setting service fees...")
        for service_type, fee in SERVICE_FEES.items():
            self.invoke("ServiceLayerGateway", "setServiceFee", service_type, fee)
            print(f"    {service_type}: {fee / 100000000} GAS")

        # 3. Register services
        print("  Registering services...")
        for service_type, contract_name in SERVICE_CONTRACTS.items():
            contract_hash = self.deployed.get(contract_name)
            if contract_hash:
                self.invoke("ServiceLayerGateway", "registerService", service_type, contract_hash)
                print(f"    {service_type} -> {contract_hash}")

    def initialize_services(self):
        """Initialize service contracts."""
        print("\n=== Initializing Service Contracts ===")

        gateway_hash = self.deployed.get("ServiceLayerGateway")
        if not gateway_hash:
            print("  Error: ServiceLayerGateway not deployed")
            return

        for service_type, contract_name in SERVICE_CONTRACTS.items():
            contract_hash = self.deployed.get(contract_name)
            if contract_hash:
                print(f"  Configuring {contract_name}...")
                self.invoke(contract_name, "setGateway", gateway_hash)

    def initialize_examples(self):
        """Initialize example consumer contracts."""
        print("\n=== Initializing Example Contracts ===")

        gateway_hash = self.deployed.get("ServiceLayerGateway")
        datafeeds_hash = self.deployed.get("DataFeedsService")

        examples = ["ExampleConsumer", "VRFLottery", "MixerClient", "DeFiPriceConsumer"]

        for example in examples:
            contract_hash = self.deployed.get(example)
            if contract_hash:
                print(f"  Configuring {example}...")
                self.invoke(example, "setGateway", gateway_hash)

                # DeFiPriceConsumer also needs DataFeeds address
                if example == "DeFiPriceConsumer" and datafeeds_hash:
                    self.invoke(example, "setDataFeedsContract", datafeeds_hash)

    def fund_accounts(self):
        """Fund test accounts with GAS for service fees."""
        print("\n=== Funding Test Accounts ===")

        if not self.network.neo_express_config:
            print("  Skipping (not Neo Express)")
            return

        import subprocess

        # Fund user account
        subprocess.run([
            "neoxp", "transfer", "100", "GAS", "genesis", "user",
            "-i", self.network.neo_express_config,
        ], capture_output=True)
        print("  Funded user account with 100 GAS")

        # Deposit to Gateway for user
        user_address = self._get_wallet_address("user")
        subprocess.run([
            "neoxp", "transfer", "10", "GAS", "user", self.deployed.get("ServiceLayerGateway", ""),
            "-i", self.network.neo_express_config,
        ], capture_output=True)
        print("  Deposited 10 GAS to Gateway for user")

    def _get_wallet_address(self, wallet_name: str) -> str:
        """Get wallet address from Neo Express."""
        import subprocess
        import json

        result = subprocess.run(
            ["neoxp", "wallet", "list", "-i", self.network.neo_express_config, "--json"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            print(f"  Warning: Failed to list wallets: {result.stderr}")
            return ""

        try:
            wallets = json.loads(result.stdout)
            for wallet in wallets:
                if wallet.get("name") == wallet_name:
                    accounts = wallet.get("accounts", [])
                    if accounts:
                        return accounts[0].get("address", "")
            return ""
        except json.JSONDecodeError:
            print(f"  Warning: Failed to parse wallet list")
            return ""

    def run(self):
        """Run full initialization."""
        print(f"=== Service Layer Initialization ({self.network.name}) ===")
        print(f"RPC URL: {self.network.rpc_url}")
        print(f"Deployed contracts: {len(self.deployed)}")

        self.initialize_gateway()
        self.initialize_services()
        self.initialize_examples()
        self.fund_accounts()

        print("\n=== Initialization Complete ===")
        print("\nDeployed contract addresses:")
        for name, hash in self.deployed.items():
            print(f"  {name}: {hash}")


def main():
    network = sys.argv[1] if len(sys.argv) > 1 else "neoexpress"

    try:
        initializer = ContractInitializer(network)
        initializer.run()
    except FileNotFoundError as e:
        print(f"Error: {e}")
        print("Run deploy_all.sh first to deploy contracts")
        sys.exit(1)
    except Exception as e:
        print(f"Error during initialization: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
