#!/usr/bin/env python3
"""
Neo MiniApp Platform Contract Initialization Script

This script initializes the **MiniApp platform** contracts after deployment:

- Sets the **Updater** for platform-write contracts to the TEE signer account.
  - `PriceFeed.setUpdater(tee)`
  - `RandomnessLog.setUpdater(tee)`
  - `AutomationAnchor.setUpdater(tee)`

The updater is expected to be the enclave-managed signer (GlobalSigner/TxProxy)
in production, but for Neo Express we use the `tee` wallet created by
`deploy/scripts/setup_neoexpress.sh`.

Usage:
  python3 deploy/scripts/initialize.py [neoexpress|testnet]
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Optional

SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
CONFIG_DIR = PROJECT_ROOT / "deploy" / "config"
DEPLOYED_FILE = CONFIG_DIR / "deployed_contracts.json"


def reverse_hash160(value: str) -> str:
    """
    Reverse a Hash160 (UInt160) hex string by bytes.

    Neo tooling is inconsistent about endianness between RPC/display and
    contract invocation arguments. For `neoxp contract run`, Hash160 arguments
    are interpreted in the opposite byte order of the deployment output.
    """
    hex_value = value[2:] if value.startswith("0x") else value
    raw = bytes.fromhex(hex_value)
    if len(raw) != 20:
        raise ValueError(f"expected 20-byte Hash160, got {len(raw)} bytes")
    return "0x" + raw[::-1].hex()


def resolve_neoxp() -> str:
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


def dotnet_env() -> Dict[str, str]:
    env = dict(os.environ)
    if env.get("DOTNET_ROOT"):
        return env

    dotnet_root = Path.home() / ".dotnet"
    if (dotnet_root / "dotnet").exists():
        env["DOTNET_ROOT"] = str(dotnet_root)
    return env


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


class PlatformInitializer:
    def __init__(self, network: str = "neoexpress"):
        cfg = NETWORKS.get(network)
        if cfg is None:
            raise ValueError(f"unknown network: {network}")
        self.network = cfg
        self.neoxp = resolve_neoxp()
        self.env = dotnet_env()
        self.deployed = self._load_deployed_contracts()

    def _load_deployed_contracts(self) -> Dict[str, str]:
        if not DEPLOYED_FILE.exists():
            raise FileNotFoundError(f"Deployed contracts file not found: {DEPLOYED_FILE}")
        return json.loads(DEPLOYED_FILE.read_text())

    def _wallet_account(self, wallet_name: str) -> Dict[str, Any]:
        if not self.network.neo_express_config:
            return {}

        result = subprocess.run(
            [self.neoxp, "wallet", "list", "-i", self.network.neo_express_config, "--json"],
            capture_output=True,
            text=True,
            env=self.env,
        )
        if result.returncode != 0:
            raise RuntimeError(f"Failed to list wallets: {result.stderr or result.stdout}")

        wallets = json.loads(result.stdout)
        entry = wallets.get(wallet_name)
        if entry is None:
            return {}
        if isinstance(entry, list):
            return entry[0] if entry else {}
        if isinstance(entry, dict):
            return entry
        return {}

    def invoke(self, contract_name: str, method: str, *args: str, signer: str = "owner") -> None:
        contract_hash = self.deployed.get(contract_name)
        if not contract_hash:
            raise RuntimeError(f"contract not found in deployed_contracts.json: {contract_name}")

        if not self.network.neo_express_config:
            raise RuntimeError("RPC-only initialization is not implemented; use neoexpress or initialize manually.")

        cmd = [
            self.neoxp,
            "contract",
            "run",
            "-i",
            self.network.neo_express_config,
            "-a",
            signer,
            contract_hash,
            method,
        ]
        cmd.extend(str(a) for a in args)

        result = subprocess.run(cmd, capture_output=True, text=True, env=self.env)
        if result.returncode != 0:
            raise RuntimeError(f"neoxp invoke failed: {result.stderr or result.stdout}")

    def set_platform_updaters(self) -> None:
        if not self.network.neo_express_config:
            return

        tee_account = self._wallet_account("tee")
        tee_hash = tee_account.get("script-hash", "")
        if not tee_hash:
            raise RuntimeError("TEE wallet not found or missing script-hash (expected wallet name: tee)")

        updater_arg = reverse_hash160(tee_hash)

        print("\n=== Setting platform Updater (TEE signer) ===")
        for contract_name in ("PriceFeed", "RandomnessLog", "AutomationAnchor"):
            if contract_name not in self.deployed:
                print(f"  - {contract_name}: not deployed, skipping")
                continue
            print(f"  - {contract_name}.setUpdater({tee_hash})")
            self.invoke(contract_name, "setUpdater", updater_arg)

    def run(self) -> None:
        if self.network.name != "neoexpress":
            raise RuntimeError(
                "Only neoexpress initialization is supported by this script.\n"
                "For testnet/mainnet: deploy contracts, then call setUpdater from the admin wallet."
            )

        self.set_platform_updaters()
        print("\n=== Initialization complete ===")


def main() -> None:
    network = sys.argv[1] if len(sys.argv) > 1 else "neoexpress"
    PlatformInitializer(network).run()


if __name__ == "__main__":
    main()
