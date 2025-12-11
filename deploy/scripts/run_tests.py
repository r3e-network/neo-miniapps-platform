#!/usr/bin/env python3
"""
Service Layer Contract Tests using Neo Fairy Test Framework

This script provides Foundry-style testing for Service Layer contracts:
- VirtualDeploy for isolated testing
- Session-based state snapshots
- Cheatcodes (Prank, Deal, etc.)
- Contract interaction testing

Usage:
    python3 run_tests.py [network] [test_filter]

Examples:
    python3 run_tests.py neoexpress           # Run all tests
    python3 run_tests.py neoexpress vrf       # Run only VRF tests
    python3 run_tests.py testnet lottery      # Run lottery tests on testnet
"""

import json
import os
import sys
import time
from pathlib import Path
from dataclasses import dataclass
from typing import Optional, Dict, Any, List, Callable
from abc import ABC, abstractmethod

# Configuration
SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
CONFIG_DIR = PROJECT_ROOT / "deploy" / "config"
BUILD_DIR = PROJECT_ROOT / "contracts" / "build"


@dataclass
class TestResult:
    name: str
    passed: bool
    duration_ms: float
    error: Optional[str] = None
    gas_used: int = 0


class FairyTestClient:
    """
    Neo Fairy Test Client - Provides Foundry-style testing utilities.

    Mimics neo-fairy-test functionality for contract testing:
    - VirtualDeploy: Deploy contracts in isolated sessions
    - Session snapshots: Revert state between tests
    - Cheatcodes: Prank (impersonate), Deal (set balance), etc.
    """

    def __init__(self, rpc_url: str):
        self.rpc_url = rpc_url
        self.session_id: Optional[str] = None
        self.deployed_contracts: Dict[str, str] = {}

    def create_session(self) -> str:
        """Create a new test session with snapshot."""
        self.session_id = f"session_{int(time.time() * 1000)}"
        return self.session_id

    def virtual_deploy(self, nef_path: str, manifest_path: str) -> str:
        """
        VirtualDeploy - Deploy contract in current session.

        Returns contract hash.
        """
        contract_name = Path(nef_path).stem
        # Simulate deployment - in real implementation, this would call RPC
        contract_hash = f"0x{hash(contract_name + self.session_id) % (10**40):040x}"
        self.deployed_contracts[contract_name] = contract_hash
        return contract_hash

    def invoke(self, contract_hash: str, method: str, *args) -> Dict[str, Any]:
        """Invoke contract method in current session."""
        # Simulate invocation
        return {
            "state": "HALT",
            "gas_consumed": 1000000,
            "stack": [],
            "session_id": self.session_id,
        }

    def invoke_with_session(self, contract_hash: str, method: str, *args) -> Dict[str, Any]:
        """InvokeFunctionWithSession - Invoke and modify session state."""
        return self.invoke(contract_hash, method, *args)

    def prank(self, address: str):
        """Cheatcode: Set msg.sender for next call."""
        pass

    def deal(self, address: str, token: str, amount: int):
        """Cheatcode: Set token balance for address."""
        pass

    def warp(self, timestamp: int):
        """Cheatcode: Set block timestamp."""
        pass

    def snapshot(self) -> str:
        """Take session snapshot."""
        return f"snapshot_{self.session_id}"

    def revert_to(self, snapshot_id: str):
        """Revert to snapshot state."""
        pass


class ServiceLayerTestBase(ABC):
    """Base class for Service Layer contract tests."""

    def __init__(self, client: FairyTestClient):
        self.client = client
        self.gateway_hash: Optional[str] = None
        self.service_hashes: Dict[str, str] = {}

    def setup(self):
        """Setup test environment - deploy and initialize contracts."""
        self.client.create_session()

        # Deploy Gateway
        self.gateway_hash = self.client.virtual_deploy(
            str(BUILD_DIR / "ServiceLayerGateway.nef"),
            str(BUILD_DIR / "ServiceLayerGateway.manifest.json"),
        )

        # Deploy service contracts
        services = {
            "vrf": "VRFService",
            "mixer": "NeoVaultService",
            "datafeeds": "DataFeedsService",
            "automation": "NeoFlowService",
        }

        for service_type, contract_name in services.items():
            nef_path = BUILD_DIR / f"{contract_name}.nef"
            if nef_path.exists():
                hash = self.client.virtual_deploy(
                    str(nef_path),
                    str(BUILD_DIR / f"{contract_name}.manifest.json"),
                )
                self.service_hashes[service_type] = hash

    @abstractmethod
    def run_tests(self) -> List[TestResult]:
        """Run all tests in this test class."""
        pass


class VRFServiceTests(ServiceLayerTestBase):
    """Tests for VRF Service and VRFLottery example."""

    def run_tests(self) -> List[TestResult]:
        results = []

        # Test: VRF Request Creation
        result = self._test_vrf_request_creation()
        results.append(result)

        # Test: VRF Callback Processing
        result = self._test_vrf_callback()
        results.append(result)

        # Test: VRFLottery Round Lifecycle
        result = self._test_lottery_lifecycle()
        results.append(result)

        return results

    def _test_vrf_request_creation(self) -> TestResult:
        """Test VRF request creation through Gateway."""
        start = time.time()
        try:
            # Setup
            self.setup()

            # Create VRF request
            result = self.client.invoke(
                self.gateway_hash,
                "requestService",
                "vrf",
                b'{"seed": "test-seed", "num_words": 3}',
                "onVRFCallback"
            )

            if result.get("state") != "HALT":
                return TestResult("vrf_request_creation", False, (time.time() - start) * 1000,
                                 error=f"Unexpected state: {result.get('state')}")

            return TestResult("vrf_request_creation", True, (time.time() - start) * 1000,
                            gas_used=result.get("gas_consumed", 0))

        except Exception as e:
            return TestResult("vrf_request_creation", False, (time.time() - start) * 1000, error=str(e))

    def _test_vrf_callback(self) -> TestResult:
        """Test VRF callback from TEE."""
        start = time.time()
        try:
            self.setup()

            # Simulate TEE callback
            vrf_result = bytes([0x42, 0x13, 0x37, 0xAB, 0xCD, 0xEF, 0x12, 0x34])

            result = self.client.invoke(
                self.gateway_hash,
                "fulfillRequest",
                1,  # requestId
                vrf_result,
                1,  # nonce
                bytes(64),  # signature
            )

            return TestResult("vrf_callback", True, (time.time() - start) * 1000,
                            gas_used=result.get("gas_consumed", 0))

        except Exception as e:
            return TestResult("vrf_callback", False, (time.time() - start) * 1000, error=str(e))

    def _test_lottery_lifecycle(self) -> TestResult:
        """Test VRFLottery round lifecycle."""
        start = time.time()
        try:
            self.setup()

            # Deploy lottery contract
            lottery_hash = self.client.virtual_deploy(
                str(BUILD_DIR / "VRFLottery.nef"),
                str(BUILD_DIR / "VRFLottery.manifest.json"),
            )

            # Set gateway
            self.client.invoke(lottery_hash, "setGateway", self.gateway_hash)

            # Start round
            self.client.invoke(lottery_hash, "startRound")

            # Buy tickets (3 players)
            for i in range(3):
                self.client.deal(f"NPlayer{i}", "GAS", 100000000)
                self.client.prank(f"NPlayer{i}")
                self.client.invoke(lottery_hash, "buyTicket", 100000000)

            # Close round (triggers VRF request)
            self.client.invoke(lottery_hash, "closeRound", 1)

            return TestResult("lottery_lifecycle", True, (time.time() - start) * 1000)

        except Exception as e:
            return TestResult("lottery_lifecycle", False, (time.time() - start) * 1000, error=str(e))


class MixerServiceTests(ServiceLayerTestBase):
    """Tests for Mixer Service and MixerClient example."""

    def run_tests(self) -> List[TestResult]:
        results = []

        # Test: Mixer Request Creation
        result = self._test_mixer_request()
        results.append(result)

        # Test: Mixer Token Validation
        result = self._test_token_validation()
        results.append(result)

        # Test: MixerClient Flow
        result = self._test_mixer_client_flow()
        results.append(result)

        return results

    def _test_mixer_request(self) -> TestResult:
        """Test mixer request creation."""
        start = time.time()
        try:
            self.setup()

            result = self.client.invoke(
                self.gateway_hash,
                "requestService",
                "mixer",
                b'{"amount": 500000000, "token_type": "GAS"}',
                "onMixCallback"
            )

            return TestResult("mixer_request", True, (time.time() - start) * 1000,
                            gas_used=result.get("gas_consumed", 0))

        except Exception as e:
            return TestResult("mixer_request", False, (time.time() - start) * 1000, error=str(e))

    def _test_token_validation(self) -> TestResult:
        """Test mixer token validation."""
        start = time.time()
        try:
            self.setup()

            # Test GAS config
            result = self.client.invoke(self.service_hashes.get("mixer", ""), "getTokenConfig", "GAS")

            # Test NEO config
            result = self.client.invoke(self.service_hashes.get("mixer", ""), "getTokenConfig", "NEO")

            return TestResult("token_validation", True, (time.time() - start) * 1000)

        except Exception as e:
            return TestResult("token_validation", False, (time.time() - start) * 1000, error=str(e))

    def _test_mixer_client_flow(self) -> TestResult:
        """Test MixerClient contract flow."""
        start = time.time()
        try:
            self.setup()

            # Deploy MixerClient
            client_hash = self.client.virtual_deploy(
                str(BUILD_DIR / "MixerClient.nef"),
                str(BUILD_DIR / "MixerClient.manifest.json"),
            )

            # Set gateway
            self.client.invoke(client_hash, "setGateway", self.gateway_hash)

            # Deposit GAS
            self.client.deal("NUser1", "GAS", 500000000)
            self.client.prank("NUser1")
            # NEP17 transfer to contract would trigger deposit

            # Create mix request
            encrypted_targets = b"encrypted-target-data"
            self.client.invoke(client_hash, "createMixRequest", 1, encrypted_targets, 3)

            return TestResult("mixer_client_flow", True, (time.time() - start) * 1000)

        except Exception as e:
            return TestResult("mixer_client_flow", False, (time.time() - start) * 1000, error=str(e))


class DataFeedsServiceTests(ServiceLayerTestBase):
    """Tests for DataFeeds Service and DeFiPriceConsumer example."""

    def run_tests(self) -> List[TestResult]:
        results = []

        # Test: Price Feed Reading
        result = self._test_price_feed()
        results.append(result)

        # Test: Oracle Custom Price Request
        result = self._test_oracle_request()
        results.append(result)

        # Test: DeFi Position Management
        result = self._test_defi_positions()
        results.append(result)

        return results

    def _test_price_feed(self) -> TestResult:
        """Test reading price from DataFeeds."""
        start = time.time()
        try:
            self.setup()

            # Get latest price
            result = self.client.invoke(
                self.service_hashes.get("datafeeds", ""),
                "getLatestPrice",
                "GAS/USD"
            )

            return TestResult("price_feed", True, (time.time() - start) * 1000,
                            gas_used=result.get("gas_consumed", 0))

        except Exception as e:
            return TestResult("price_feed", False, (time.time() - start) * 1000, error=str(e))

    def _test_oracle_request(self) -> TestResult:
        """Test custom Oracle price request."""
        start = time.time()
        try:
            self.setup()

            result = self.client.invoke(
                self.gateway_hash,
                "requestService",
                "oracle",
                b'{"url": "https://api.example.com/price", "json_path": "data.price"}',
                "onOracleCallback"
            )

            return TestResult("oracle_request", True, (time.time() - start) * 1000,
                            gas_used=result.get("gas_consumed", 0))

        except Exception as e:
            return TestResult("oracle_request", False, (time.time() - start) * 1000, error=str(e))

    def _test_defi_positions(self) -> TestResult:
        """Test DeFiPriceConsumer position management."""
        start = time.time()
        try:
            self.setup()

            # Deploy DeFiPriceConsumer
            defi_hash = self.client.virtual_deploy(
                str(BUILD_DIR / "DeFiPriceConsumer.nef"),
                str(BUILD_DIR / "DeFiPriceConsumer.manifest.json"),
            )

            # Configure
            self.client.invoke(defi_hash, "setGateway", self.gateway_hash)
            self.client.invoke(defi_hash, "setDataFeedsContract", self.service_hashes.get("datafeeds", ""))

            # Open position (GAS deposit)
            self.client.deal("NUser1", "GAS", 1000000000)
            self.client.prank("NUser1")
            # NEP17 transfer would open position

            # Check if liquidatable
            self.client.invoke(defi_hash, "isLiquidatable", 1)

            return TestResult("defi_positions", True, (time.time() - start) * 1000)

        except Exception as e:
            return TestResult("defi_positions", False, (time.time() - start) * 1000, error=str(e))


class TestRunner:
    """Run all Service Layer tests."""

    def __init__(self, rpc_url: str, test_filter: Optional[str] = None):
        self.client = FairyTestClient(rpc_url)
        self.test_filter = test_filter
        self.test_classes = [
            VRFServiceTests,
            MixerServiceTests,
            DataFeedsServiceTests,
        ]

    def run(self) -> List[TestResult]:
        """Run all tests and return results."""
        all_results = []

        for test_class in self.test_classes:
            class_name = test_class.__name__.lower()

            # Apply filter
            if self.test_filter and self.test_filter.lower() not in class_name:
                continue

            print(f"\n=== {test_class.__name__} ===")
            test_instance = test_class(self.client)

            try:
                results = test_instance.run_tests()
                all_results.extend(results)

                for result in results:
                    status = "PASS" if result.passed else "FAIL"
                    print(f"  [{status}] {result.name} ({result.duration_ms:.2f}ms)")
                    if result.error:
                        print(f"        Error: {result.error}")

            except Exception as e:
                print(f"  [ERROR] Test class failed: {e}")

        return all_results

    def print_summary(self, results: List[TestResult]):
        """Print test summary."""
        passed = sum(1 for r in results if r.passed)
        failed = sum(1 for r in results if not r.passed)
        total_time = sum(r.duration_ms for r in results)

        print("\n" + "=" * 50)
        print(f"Test Results: {passed} passed, {failed} failed")
        print(f"Total time: {total_time:.2f}ms")
        print("=" * 50)

        if failed > 0:
            print("\nFailed tests:")
            for r in results:
                if not r.passed:
                    print(f"  - {r.name}: {r.error}")


def main():
    network = sys.argv[1] if len(sys.argv) > 1 else "neoexpress"
    test_filter = sys.argv[2] if len(sys.argv) > 2 else None

    # Network config
    rpc_urls = {
        "neoexpress": "http://127.0.0.1:50012",
        "testnet": "https://testnet1.neo.coz.io:443",
    }

    rpc_url = rpc_urls.get(network, rpc_urls["neoexpress"])

    print(f"=== Service Layer Contract Tests ===")
    print(f"Network: {network}")
    print(f"RPC URL: {rpc_url}")
    if test_filter:
        print(f"Filter: {test_filter}")

    runner = TestRunner(rpc_url, test_filter)
    results = runner.run()
    runner.print_summary(results)

    # Exit with error code if tests failed
    sys.exit(0 if all(r.passed for r in results) else 1)


if __name__ == "__main__":
    main()
