# Devpack SDK Changelog

All SDKs are aligned to the Devpack runtime surface. Version numbers below refer
to the TypeScript package (`@service-layer/devpack`) and mirror releases for Go,
Rust, and Python helpers where applicable.

## 0.6.0
- Added DataLink delivery helper (`datalink.createDelivery`).
- Added data feed submission helper (`datafeeds.submitUpdate`) and data stream
  frame publishing helper (`datastreams.publishFrame`).
- Added randomness helper (`random.generate`) with result encoding metadata.
- Updated function service to execute data feeds, data streams, datalink, and
  random actions.
- Added polyglot helper packages under `sdk/go/devpack`, `sdk/rust/devpack`,
  and `sdk/python/devpack`.

## 0.5.x and earlier
- Initial TypeScript helpers for gas bank, oracle, price feed snapshots, triggers,
  and automation scheduling.
