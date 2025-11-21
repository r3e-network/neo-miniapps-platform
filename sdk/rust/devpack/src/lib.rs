use serde::{Deserialize, Serialize};
use std::collections::HashMap;

pub const ACTION_GASBANK_ENSURE: &str = "gasbank.ensureAccount";
pub const ACTION_GASBANK_WITHDRAW: &str = "gasbank.withdraw";
pub const ACTION_GASBANK_BALANCE: &str = "gasbank.balance";
pub const ACTION_GASBANK_LIST: &str = "gasbank.listTransactions";
pub const ACTION_ORACLE_CREATE: &str = "oracle.createRequest";
pub const ACTION_PRICEFEED_SNAPSHOT: &str = "pricefeed.recordSnapshot";
pub const ACTION_RANDOM_GENERATE: &str = "random.generate";
pub const ACTION_DATAFEED_SUBMIT: &str = "datafeeds.submitUpdate";
pub const ACTION_DATASTREAM_PUBLISH: &str = "datastreams.publishFrame";
pub const ACTION_DATALINK_CREATE: &str = "datalink.createDelivery";
pub const ACTION_TRIGGERS_REGISTER: &str = "triggers.register";
pub const ACTION_AUTOMATION_SCHEDULE: &str = "automation.schedule";

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct Action {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    pub r#type: String,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub params: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ActionRef {
    pub __devpack_ref__: bool,
    pub id: String,
    pub r#type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Response {
    pub success: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<serde_json::Value>,
}

fn action(t: &str, params: Option<HashMap<String, serde_json::Value>>) -> Action {
    Action {
        id: None,
        r#type: t.to_string(),
        params: params.unwrap_or_default(),
    }
}

pub fn ensure_gas_account(params: Option<HashMap<String, serde_json::Value>>) -> Action {
    action(ACTION_GASBANK_ENSURE, params)
}

pub fn withdraw_gas(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_GASBANK_WITHDRAW, Some(params))
}

pub fn balance_gas_account(params: Option<HashMap<String, serde_json::Value>>) -> Action {
    action(ACTION_GASBANK_BALANCE, params)
}

pub fn list_gas_transactions(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_GASBANK_LIST, Some(params))
}

pub fn create_oracle_request(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_ORACLE_CREATE, Some(params))
}

pub fn record_price_snapshot(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_PRICEFEED_SNAPSHOT, Some(params))
}

pub fn generate_random(params: Option<HashMap<String, serde_json::Value>>) -> Action {
    let mut p = params.unwrap_or_default();
    if !p.contains_key("length") {
        p.insert("length".into(), serde_json::json!(32));
    }
    action(ACTION_RANDOM_GENERATE, Some(p))
}

pub fn submit_datafeed_update(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_DATAFEED_SUBMIT, Some(params))
}

pub fn publish_datastream_frame(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_DATASTREAM_PUBLISH, Some(params))
}

pub fn create_datalink_delivery(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_DATALINK_CREATE, Some(params))
}

pub fn register_trigger(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_TRIGGERS_REGISTER, Some(params))
}

pub fn schedule_automation(params: HashMap<String, serde_json::Value>) -> Action {
    action(ACTION_AUTOMATION_SCHEDULE, Some(params))
}

pub fn as_result(action: &Action, meta: Option<HashMap<String, serde_json::Value>>) -> ActionRef {
    ActionRef {
        __devpack_ref__: true,
        id: action.id.clone().unwrap_or_default(),
        r#type: action.r#type.clone(),
        meta,
    }
}

pub fn success(data: Option<serde_json::Value>, meta: Option<serde_json::Value>) -> Response {
    Response {
        success: true,
        data,
        error: None,
        meta,
    }
}

pub fn failure(error: Option<serde_json::Value>, meta: Option<serde_json::Value>) -> Response {
    Response {
        success: false,
        data: None,
        error,
        meta,
    }
}
