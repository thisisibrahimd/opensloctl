# `api-latency-slo` - API latency with single-window burn-rate alerting

Demonstrates the [`burn-rate`](../README.md#burn-rate--sre-%C2%A74) alerting strategy (SRE Workbook § 4). One burn-rate condition per severity, no tiering.

## Strategy

The simplest meaningful burn-rate alert. Each severity has a single burn-rate multiplier over a single lookback window. No OR-ed windows, no AND-ed tiers - just one threshold and one window per severity.

## Files

```
api-latency-slo/
├── service.yaml                       # "api-gateway" service
├── datasource.yaml                    # Prometheus datasource
├── sli.yaml                           # thresholdMetric: P99 latency (histogram_quantile)
├── alert-condition-page.yaml          # kind: burn-rate, threshold: 14.4, window: 5m
├── alert-condition-ticket.yaml        # kind: burn-rate, threshold: 3, window: 2h
├── alert-policy-page.yaml             # page → pagerduty
├── alert-policy-ticket.yaml           # ticket → slack
├── notification-target-pagerduty.yaml
├── notification-target-slack.yaml
└── slo.yaml                           # 99.9% P99 < 500ms, refs both policies
```

## Spec diagram

```text
                                              ┌─────────────────────┐
                                              │ AlertNotification   │
                                              │ Target: pagerduty   │
                                              │ name: oncall-pd     │
                                              └──────────▲──────────┘
                                                         │ targetRef
┌──────────────────┐  spec.alertPolicies[0]   ┌──────────┴──────────┐
│ AlertCondition   │  ──────────────────────▶│ AlertPolicy         │
│ severity: page   │                         │ api-latency-page-…  │
│ threshold: 14.4  │                         │ conditions[0]       │
│ window: 5m       │                         │ notification: pd    │
│ kind: burn-rate  │                         └─────────────────────┘
└─▲────────────────┘
  │ conditionRef
  │
┌─┴──────────────┐  indicatorRef           ┌──────────────────────┐
│ AlertCond.     │  (note: SLI uses        │ SLO                  │
│  api-latency-  │   thresholdMetric,       │ api-latency-slo      │
│  page          │   not ratioMetric)       │ budgetMethod:        │
└───────────────┘                          │  Occurrences         │
                                           │ window: 30d rolling  │
                                           │ objective: P99<500ms │
                                           │ alertPolicies: …     │
                                           └──────────────────────┘
```

## Generated rules (highlights)

```
opensloctl generate -f examples/api-latency-slo -o output/
```

The generator renders `api-latency-slo-rules.yaml` containing the SLO info recordings, windowed SLI recordings (P99 over 5m, 30m, 1h, …) and an `openslo-alerts-api-latency-slo` group with a single alert per severity:

```yaml
- alert: ApiLatencySloBurnRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 14.400000
  for: 2m
  labels:
    severity: page
- alert: ApiLatencySloBurnRate
  expr: openslo_sli_error_rate_2h{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 3.000000
  for: 15m
  labels:
    severity: ticket
```

## Run

```
opensloctl generate -f examples/api-latency-slo -o output/
ls output/
# api-latency-slo-rules.yaml
```

## When to use this kind

`burn-rate` is the simplest burn-based alert - useful when you only want one threshold per severity and don't need the smoothing that paired short+long windows provide. For better noise immunity, upgrade to [`multi-window-multi-burn-rate`](../README.md#multi-window-multi-burn-rate--sre-%C2%A76-recommended).
