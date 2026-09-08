# `error-budget-slo` - Checkout success rate with single-window burn-rate alerting

Sibling of [`api-latency-slo`](../api-latency-slo/README.md), but with a ratioMetric SLI (good/total requests) instead of a thresholdMetric (latency histogram). Both examples exercise the [`burn-rate`](../README.md#burn-rate--sre-%C2%A74) alerting strategy.

## Strategy

Same as `api-latency-slo`: one `burn-rate` condition per severity. Use this example when you have a counter-style SLI (success/total request counts) rather than a latency histogram.

## Files

```
error-budget-slo/
├── service.yaml                       # "checkout-service"
├── datasource.yaml
├── sli.yaml                           # ratioMetric: 2xx / total http requests
├── alert-condition-page.yaml          # kind: burn-rate, threshold: 14.4, window: 5m
├── alert-condition-ticket.yaml        # kind: burn-rate, threshold: 3, window: 2h
├── alert-policy-page.yaml             # page → pagerduty
├── alert-policy-ticket.yaml           # ticket → slack
├── notification-target-pagerduty.yaml
├── notification-target-slack.yaml
└── slo.yaml                           # 99.9% checkout success over 30d
```

## Spec diagram

```text
┌──────────────────┐  spec.alertPolicies[0]   ┌──────────────────────┐
│ AlertCondition   │  ──────────────────────▶│ AlertPolicy          │
│ severity: page   │                         │ checkout-page-alert  │
│ threshold: 14.4  │                         │ notification: pd     │
│ window: 5m       │                         └──────────────────────┘
│ kind: burn-rate  │
└─▲────────────────┘
  │ conditionRef
  │
┌─┴──────────────┐                         ┌──────────────────────┐
│ AlertCond.     │                         │ SLO                  │
│ checkout-page  │ ───── indicator ──────▶ │ checkout-slo         │
└───────────────┘                          │ ratioMetric:         │
                                           │  good / total        │
                                           │ objective: 99.9%     │
                                           │ alertPolicies: …     │
                                           └──────────────────────┘
```

## Generated rules (highlights)

```
opensloctl generate -f examples/error-budget-slo -o output/
```

The generator renders `checkout-slo-rules.yaml` with the SLO info recordings, the windowed goodness recordings (`openslo_sli_error_rate_*`) and an `openslo-alerts-checkout-slo` group with two burn-rate alerts:

```yaml
- alert: CheckoutSloBurnRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="checkout-slo"} / (1 - openslo_slo_objective{openslo_slo_name="checkout-slo"}) >= 14.400000
  for: 2m
  labels:
    severity: page
- alert: CheckoutSloBurnRate
  expr: openslo_sli_error_rate_2h{openslo_slo_name="checkout-slo"} / (1 - openslo_slo_objective{openslo_slo_name="checkout-slo"}) >= 3.000000
  for: 15m
  labels:
    severity: ticket
```

## When to use this kind

Same as `api-latency-slo`: `burn-rate` is the simplest burn-based alert. The choice of SLI source (ratioMetric here vs thresholdMetric in api-latency-slo) doesn't change the alert strategy - pick whichever fits your data.
