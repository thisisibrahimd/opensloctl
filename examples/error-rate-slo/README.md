# `error-rate-slo` - Checkout availability with raw error-rate alerting

Demonstrates the [`error-rate`](../README.md#error-rate--sre-%C2%A7%C2%A7-13) alerting strategy. The simplest possible setup: one `error-rate` condition per severity, comparing the SLI error rate directly to an absolute threshold.

## Strategy

From the SRE Workbook §§ 1–3, the "target error rate" / "increased alert window" / "alert on incrementing duration" patterns all share the same expression shape - the only differences are:

1. `lookbackWindow` length (short vs long)
2. Whether `alertAfter` is set (maps to Prom `for:`)

So we collapse all three into a single kind: `error-rate`.

## Files

```
error-rate-slo/
├── service.yaml                       # "checkout" service
├── sli.yaml                           # ratioMetric: success / total checkout requests
├── alert-condition-page.yaml          # kind: error-rate, threshold: 0.001, window 5m
├── alert-condition-ticket.yaml        # kind: error-rate, threshold: 0.005, window 1h
├── alert-policy-page.yaml             # 1 condition per policy (SDK rule)
├── alert-policy-ticket.yaml
├── notification-target-pagerduty.yaml # page → pagerduty
├── notification-target-slack.yaml     # ticket → slack
└── slo.yaml                           # 99.9% availability over 30d, refs both policies
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
│ RuleCondition    │  ──────────────────────▶│ AlertPolicy         │
│ severity: page   │                         │ name:               │
│ threshold: 0.001 │                         │  checkout-page-…    │
│ window: 5m       │                         │ conditions[0].cond… │
│ kind: error-rate │                         │  → checkout-page    │
└───▲──────────────┘                         └─────────────────────┘
    │ conditionRef
    │
┌───┴──────────┐  spec.indicatorRef        ┌──────────────────────┐
│ AlertCond.   │                          │ SLO                  │
│ checkout-page│ ────────────── uses ────▶ │ checkout-availability│
└──────────────┘                          │ service: checkout    │
     │                                    │ budgetMethod:        │
     │ uses                               │  Occurrences         │
     ▼                                    │ window: 30d rolling  │
┌──────────────┐                          │ objective: 0.999     │
│ SLI          │                          │ alertPolicies:       │
│ checkout-…   │                          │  → checkout-page-alert│
└──────────────┘                          │  → checkout-ticket-… │
                                           └──────────────────────┘
```

## Generated rules (highlights)

The generator renders a single file `checkout-availability-rules.yaml` containing both recording rules and an `openslo-alerts-checkout-availability` group. The alert group has two `error-rate` alerts (one per severity, same name).

```yaml
- alert: CheckoutAvailabilityErrorRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="checkout-availability"} >= 0.001000
  for: 2m
  labels:
    severity: page
- alert: CheckoutAvailabilityErrorRate
  expr: openslo_sli_error_rate_1h{openslo_slo_name="checkout-availability"} >= 0.005000
  for: "
  labels:
    severity: ticket
```

Validation that fires at load time:

- `kind: error-rate` (accepted ✓)
- `threshold: 0.001` and `threshold: 0.005` both in `(0, 1]` ✓

## Run

```
opensloctl generate -f examples/error-rate-slo -o output/
ls output/
# checkout-availability-rules.yaml
```

## When to use this kind

Pick `error-rate` when you want the simplest possible burn-free alerting setup. The threshold is the **error rate itself**, not a burn multiplier - easy to reason about, but lacks the smoothing that burn rates provide against short bursts.
