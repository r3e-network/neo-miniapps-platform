# Service Layer Alerting Examples

These PromQL snippets pair with the engine metrics emitted from `/metrics`:

## Dependency waits
Alert when any module is blocked on dependencies for more than a few minutes.
```
engine_module_waiting_dependencies = service_layer_engine_module_waiting_dependencies

alert: ModuleWaitingOnDeps
expr: max_over_time(engine_module_waiting_dependencies[5m]) > 0
for: 5m
labels:
  severity: warning
annotations:
  summary: "Module waiting on dependencies"
  description: "{{ $labels.module }} (domain={{ $labels.domain }}) is blocked on dependencies for 5m+"
```

## Not-ready modules
Alert when a module loses readiness.
```
engine_module_ready = service_layer_engine_module_ready

alert: ModuleNotReady
expr: engine_module_ready == 0
for: 2m
labels:
  severity: critical
annotations:
  summary: "Module not ready"
  description: "{{ $labels.module }} (domain={{ $labels.domain }}) is not ready"
```

## Failed lifecycle
Alert when a module reports failed/stop-error status.
```
engine_module_status = service_layer_engine_module_status

alert: ModuleFailed
expr: engine_module_status{status=~"failed|stop-error"} == 1
for: 1m
labels:
  severity: critical
annotations:
  summary: "Module lifecycle failure"
  description: "{{ $labels.module }} (domain={{ $labels.domain }}) status={{ $labels.status }}"
```

## Slow starts/stops
Alert when start/stop durations exceed a threshold (seconds).
```
engine_module_start_seconds = service_layer_engine_module_start_seconds
engine_module_stop_seconds = service_layer_engine_module_stop_seconds

alert: ModuleSlowStartStop
expr: (engine_module_start_seconds > 1) or (engine_module_stop_seconds > 1)
for: 1m
labels:
  severity: warning
annotations:
  summary: "Module slow start/stop"
  description: "{{ $labels.module }} (domain={{ $labels.domain }}) start={{ $value }}s or stop exceeded threshold"
```

## Scrape config example
Add a scrape job to Prometheus to collect these metrics (replace the target with your Service Layer address):
```
scrape_configs:
  - job_name: 'service-layer'
    scrape_interval: 15s
    metrics_path: /metrics
    static_configs:
      - targets: ['service-layer:8080']
    scheme: http
    bearer_token: '<api-token-if-needed>'
    # If auth is required, you can also set basic_auth or headers; adjust as necessary.
```

## Bus fan-out failures
Alert when event/data/compute fan-outs return errors.
```
engine_bus_fanout_total = service_layer_engine_bus_fanout_total

alert: BusFanoutErrors
expr: increase(engine_bus_fanout_total{result="error"}[5m]) > 0
for: 1m
labels:
  severity: warning
annotations:
  summary: "Bus fan-out failures"
  description: "Errors detected in bus fan-out calls (kind={{ $labels.kind }})"
```
