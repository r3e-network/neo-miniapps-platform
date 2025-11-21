"""
Devpack Python SDK (experimental).

Provides lightweight helpers to emit action payloads matching the Service Layer
Devpack contract. This is a thin data model, not an HTTP client; the runtime
is expected to collect these actions from the function environment.
"""

from dataclasses import dataclass
from typing import Any, Dict, Optional

ACTION_GASBANK_ENSURE = "gasbank.ensureAccount"
ACTION_GASBANK_WITHDRAW = "gasbank.withdraw"
ACTION_GASBANK_BALANCE = "gasbank.balance"
ACTION_GASBANK_LIST = "gasbank.listTransactions"
ACTION_ORACLE_CREATE = "oracle.createRequest"
ACTION_PRICEFEED_SNAPSHOT = "pricefeed.recordSnapshot"
ACTION_RANDOM_GENERATE = "random.generate"
ACTION_DATAFEED_SUBMIT = "datafeeds.submitUpdate"
ACTION_DATASTREAM_PUBLISH = "datastreams.publishFrame"
ACTION_DATALINK_CREATE = "datalink.createDelivery"
ACTION_TRIGGERS_REGISTER = "triggers.register"
ACTION_AUTOMATION_SCHEDULE = "automation.schedule"


@dataclass
class Action:
    type: str
    params: Dict[str, Any]
    id: Optional[str] = None

    def as_result(self, meta: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        ref = {"__devpack_ref__": True, "id": self.id or "", "type": self.type}
        if meta:
            ref["meta"] = meta
        return ref


def _action(action_type: str, params: Optional[Dict[str, Any]]) -> Action:
    return Action(type=action_type, params=params or {})


def ensure_gas_account(params: Optional[Dict[str, Any]] = None) -> Action:
    return _action(ACTION_GASBANK_ENSURE, params)


def withdraw_gas(params: Dict[str, Any]) -> Action:
    return _action(ACTION_GASBANK_WITHDRAW, params)


def balance_gas_account(params: Optional[Dict[str, Any]] = None) -> Action:
    return _action(ACTION_GASBANK_BALANCE, params)


def list_gas_transactions(params: Dict[str, Any]) -> Action:
    return _action(ACTION_GASBANK_LIST, params)


def create_oracle_request(params: Dict[str, Any]) -> Action:
    return _action(ACTION_ORACLE_CREATE, params)


def record_price_snapshot(params: Dict[str, Any]) -> Action:
    return _action(ACTION_PRICEFEED_SNAPSHOT, params)


def generate_random(params: Optional[Dict[str, Any]] = None) -> Action:
    payload = params or {}
    if not payload.get("length"):
        payload["length"] = 32
    return _action(ACTION_RANDOM_GENERATE, payload)


def submit_datafeed_update(params: Dict[str, Any]) -> Action:
    return _action(ACTION_DATAFEED_SUBMIT, params)


def publish_datastream_frame(params: Dict[str, Any]) -> Action:
    return _action(ACTION_DATASTREAM_PUBLISH, params)


def create_datalink_delivery(params: Dict[str, Any]) -> Action:
    return _action(ACTION_DATALINK_CREATE, params)


def register_trigger(params: Dict[str, Any]) -> Action:
    return _action(ACTION_TRIGGERS_REGISTER, params)


def schedule_automation(params: Dict[str, Any]) -> Action:
    return _action(ACTION_AUTOMATION_SCHEDULE, params)


def success(data: Any = None, meta: Any = None) -> Dict[str, Any]:
    return {"success": True, "data": data, "meta": meta}


def failure(error: Any = None, meta: Any = None) -> Dict[str, Any]:
    return {"success": False, "error": error, "meta": meta}
