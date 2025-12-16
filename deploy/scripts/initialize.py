#!/usr/bin/env python3
"""
Service Layer Contract Initialization Script

This script initializes all deployed Service Layer contracts:
1. Registers TEE accounts with the Gateway
2. Registers request/response services with the Gateway
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
import shutil
from pathlib import Path
from dataclasses import dataclass
from typing import Optional, Dict, Any

# Configuration
SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
CONFIG_DIR = PROJECT_ROOT / "deploy" / "config"
DEPLOYED_FILE = CONFIG_DIR / "deployed_contracts.json"

def reverse_hash160(value: str) -> str:
    """
    Reverse a Hash160 (UInt160) hex string by bytes.

    Neo tooling is inconsistent about endianness between RPC/display and
    contract invocation arguments. For neoxp `contract run`, Hash160 arguments
    are interpreted in the opposite byte order of the deployment output.

    This helper normalizes by taking a 20-byte hex string and reversing bytes.
    """
    if not value:
        return ""
    hex_value = value[2:] if value.startswith("0x") else value
    raw = bytes.fromhex(hex_value)
    if len(raw) != 20:
        raise ValueError(f"expected 20-byte Hash160, got {len(raw)} bytes")
    return "0x" + raw[::-1].hex()

# Request/response services that are routed through the Gateway.
# NOTE: DataFeeds is a push-style contract and is not invoked via Gateway.requestService.
GATEWAY_SERVICES = {
    "oracle": "OracleService",
    "vrf": "VRFService",
    "automation": "NeoFlowService",
    "confidential": "ConfidentialService",
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
        self.neoxp = self._resolve_neoxp()
        self.dotnet_env = self._resolve_dotnet_env()
        self.tee_pubkey = self._get_tee_pubkey()

    def _resolve_neoxp(self) -> str:
        """Resolve the neoxp binary path (supports dotnet-tool installs)."""
        override = os.environ.get("NEOXP", "neoxp")
        resolved = shutil.which(override)
        if resolved:
            return resolved

        dotnet_tool = Path.home() / ".dotnet" / "tools" / "neoxp"
        if dotnet_tool.exists():
            return str(dotnet_tool)

        raise FileNotFoundError(
            "neoxp not found. Install with `dotnet tool install -g Neo.Express` "
            "and ensure `$HOME/.dotnet/tools` is on PATH."
        )

    def _resolve_dotnet_env(self) -> Dict[str, str]:
        """Ensure DOTNET_ROOT is set when using dotnet-local installs (~/.dotnet)."""
        env = dict(os.environ)
        if env.get("DOTNET_ROOT"):
            return env

        dotnet_root = Path.home() / ".dotnet"
        if (dotnet_root / "dotnet").exists():
            env["DOTNET_ROOT"] = str(dotnet_root)
        return env

    def _load_deployed_contracts(self) -> Dict[str, str]:
        """Load deployed contract addresses."""
        if not DEPLOYED_FILE.exists():
            raise FileNotFoundError(f"Deployed contracts file not found: {DEPLOYED_FILE}")

        with open(DEPLOYED_FILE) as f:
            return json.load(f)

    def _get_tee_pubkey(self) -> str:
        """Get TEE account public key."""
        if self.network.neo_express_config:
            # For Neo Express, resolve from `neoxp wallet list --json` (non-interactive).
            return self._get_wallet_pubkey("tee")
        return os.environ.get("TEE_PUBKEY", "")

    def _get_wallet_pubkey(self, wallet_name: str) -> str:
        """Get compressed public key from Neo Express wallet (hex, 33 bytes)."""
        import subprocess
        account = self._get_wallet_account(wallet_name)
        if not account:
            return ""
        return account.get("public-key", "")

    def _get_wallet_account(self, wallet_name: str) -> Dict[str, Any]:
        """Get the first account from a Neo Express wallet."""
        import subprocess

        result = subprocess.run(
            [self.neoxp, "wallet", "list", "-i", self.network.neo_express_config, "--json"],
            capture_output=True,
            text=True,
            env=self.dotnet_env,
        )
        if result.returncode != 0:
            raise RuntimeError(f"Failed to list wallets: {result.stderr or result.stdout}")

        try:
            wallets = json.loads(result.stdout)
            wallet_entry = wallets.get(wallet_name)
            if wallet_entry is None:
                return {}

            # Neo Express JSON shape:
            # - most wallets: { "<wallet>": [ {account}, ... ] }
            # - genesis: { "genesis": {account} }
            if isinstance(wallet_entry, list):
                return wallet_entry[0] if wallet_entry else {}
            if isinstance(wallet_entry, dict):
                return wallet_entry
            return {}
        except json.JSONDecodeError:
            print("  Warning: Failed to parse wallet list JSON")
            return {}

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

        cmd = [
            self.neoxp, "contract", "run",
            "-i", self.network.neo_express_config,
            "-a", "owner",
            contract_hash, method,
        ]
        cmd.extend(str(a) for a in args)

        result = subprocess.run(cmd, capture_output=True, text=True, env=self.dotnet_env)
        if result.returncode != 0:
            raise RuntimeError(f"neoxp invoke failed: {result.stderr or result.stdout}")
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
        tee_account = self._get_wallet_account("tee")
        tee_hash = tee_account.get("script-hash", "")
        if not tee_hash or not self.tee_pubkey:
            print("  Warning: Missing TEE wallet info (script-hash/public-key); skipping TEE registration")
        else:
            tee_pubkey_arg = self.tee_pubkey
            if tee_pubkey_arg and not tee_pubkey_arg.startswith("0x"):
                tee_pubkey_arg = f"0x{tee_pubkey_arg}"
            # Hash160 args passed via neoxp require byte-reversal to match the VM's internal ordering.
            self.invoke("ServiceLayerGateway", "registerTEEAccount", reverse_hash160(tee_hash), tee_pubkey_arg)

        # 2. Register services
        print("  Registering services...")
        for service_type, contract_name in GATEWAY_SERVICES.items():
            contract_hash = self.deployed.get(contract_name)
            if contract_hash:
                self.invoke("ServiceLayerGateway", "registerService", service_type, reverse_hash160(contract_hash))
                print(f"    {service_type} -> {contract_hash}")

    def initialize_services(self):
        """Initialize service contracts."""
        print("\n=== Initializing Service Contracts ===")

        gateway_hash = self.deployed.get("ServiceLayerGateway")
        if not gateway_hash:
            print("  Error: ServiceLayerGateway not deployed")
            return
        gateway_arg = reverse_hash160(gateway_hash)

        for service_type, contract_name in GATEWAY_SERVICES.items():
            contract_hash = self.deployed.get(contract_name)
            if contract_hash:
                print(f"  Configuring {contract_name}...")
                self.invoke(contract_name, "setGateway", gateway_arg)

    def initialize_examples(self):
        """Initialize example consumer contracts."""
        print("\n=== Initializing Example Contracts ===")

        gateway_hash = self.deployed.get("ServiceLayerGateway")
        datafeeds_hash = self.deployed.get("DataFeedsService")
        gateway_arg = reverse_hash160(gateway_hash) if gateway_hash else ""
        datafeeds_arg = reverse_hash160(datafeeds_hash) if datafeeds_hash else ""

        examples = ["ExampleConsumer", "VRFLottery", "DeFiPriceConsumer"]

        for example in examples:
            contract_hash = self.deployed.get(example)
            if contract_hash:
                print(f"  Configuring {example}...")
                self.invoke(example, "setGateway", gateway_arg)

                # DeFiPriceConsumer also needs DataFeeds address
                if example == "DeFiPriceConsumer" and datafeeds_arg:
                    self.invoke(example, "setDataFeedsContract", datafeeds_arg)

    def fund_accounts(self):
        """Fund test accounts with GAS for service fees."""
        print("\n=== Funding Test Accounts ===")

        if not self.network.neo_express_config:
            print("  Skipping (not Neo Express)")
            return

        import subprocess

        # Fund user account
        result = subprocess.run([
            self.neoxp, "transfer", "100", "GAS", "genesis", "user",
            "-i", self.network.neo_express_config,
        ], capture_output=True, env=self.dotnet_env, text=True)
        if result.returncode != 0:
            print(f"  Warning: Failed to fund user: {result.stderr or result.stdout}")
        print("  Funded user account with 100 GAS")

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
