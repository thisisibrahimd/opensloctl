# `multi-burn-slo` - Payment availability with multi-burn-rate alerting

Demonstrates the [`multi-burn-rate`](../README.md#multi-burn-rate--sre-%C2%A75) alerting strategy (SRE Workbook § 5). Multiple burn rate windows OR-ed together per severity - no short/long AND pairing.

## Strategy

Two or more burn rate windows per severity. Each condition contributes one expression to the alert. The alert fires when **any** condition fires (OR). There's no AND tiering like the multi-window variant.

A typical setup mirrors the [Burn Rate Alerts table from the SRE Workbook](https://sre.google/workbook/alerting-on-slos/#5-multiple-burn-rate-alerts):

| Severity | Condition | Lookback | Threshold | Time to burn half the budget |
|---|---|---|---|---|
| page | 36x | 5 m | fastest possible spike detection | min |
| page | 6x | 30 m | catches sustained spike | 5 h |
| ticket | 3x | 2 h | moderate sustained rate | 10 h |
| ticket | 1x | 6 h | the slow burn at the SLO rate | 30 d |

## Files

```
multi-burn-slo/
├── service.yaml                       # "payments" service
├── sli.yaml                           # ratioMetric: success / total payment requests
├── alert-condition-36x.yaml           # condition: kind: multi-burn-rate, 36x / 5m
├── alert-condition-6x.yaml            # condition: kind: multi-burn-rate, 6x / 30m
├── alert-condition-3x.yaml            # condition: kind: multi-burn-rate, 3x / 2h (ticket)
├── alert-condition-1x.yaml            # condition: kind: multi-burn-rate, 1x / 6h (ticket)
├── alert-policy-36x.yaml              # 1 condition per policy (SDK rule)
├── alert-policy-6x.yaml
├── alert-policy-3x.yaml
├── alert-policy-1x.yaml
├── notification-target-pagerduty.yaml # page → pagerduty
├── notification-target-slack.yaml     # ticket → slack
└── slo.yaml                           # 99.95% over 30d, refs all four policies
```

## Spec diagram

```text
   page severity                            ticket severity
 ─────────────────────                ────────────────────────
 ┌────────────────────┐                ┌────────────────────┐
 │ AlertCondition     │                │ AlertCondition     │
 │ name:              │                │ name:              │
 │  payment-page-36x  │                │  payment-ticket-3x │
 │ threshold: 36      │                │ threshold: 3       │
 │ window: 5m         │                │ window: 2h         │
 │ kind: multi-burn…  │                │ kind: multi-burn…  │
 └─▲─────────┬────────┘                └─▲─────────┬────────┘
   │         │ conditionRef               │         │ conditionRef
   │         ▼                            │         ▼
 ┌─┴──────────────┐ ◀── AlertPolicy ──▶ ┌─┴──────────────┐
 │ AlertPolicy    │  payment-page-36x…  │ AlertPolicy    │
 │  payment-…-36x │                     │  payment-…-3x  │
 │ notification:  │                     │ notification:  │
 │   pagerduty    │                     │   slack        │
 └────────────────┘                     └────────────────┘

  6x condition + policy are also under "page"; 1x under "ticket".
  All four are referenced from the same SLO via spec.alertPolicies.
```

## Generated rules (highlights)

The generator renders a single file `payment-availability-rules.yaml` containing recording rules plus the `openslo-alerts-payment-availability` group with two `MultiBurnRate` alerts (one per severity):

```yaml
- alert: PaymentAvailabilityMultiBurnRate           # page
  expr: |-
    (openslo_sli_error_rate_5m{openslo_slo_name="payment-availability"} / (1 - openslo_slo_objective{openslo_slo_name="payment-availability"}) >= 36.000000)
    or
    (openslo_sli_error_rate_30m{openslo_slo_name="payment-availability"} / (1 - openslo_slo_objective{openslo_slo_name="payment-availability"}) >= 6.000000)
  labels:
    severity: page

- alert: PaymentAvailabilityMultiBurnRate           # ticket
  expr: |-
    (openslo_sli_error_rate_2h{openslo_slo_name="payment-availability"} / (1 - openslo_slo_objective{openslo_slo_name="payment-availability"}) >= 3.000000)
    or
    (openslo_sli_error_rate_6h{openslo_slo_name="payment-availability"} / (1 - openslo_slo_objective{openslo_slo_name="payment-availability"}) >= 1.000000)
  labels:
    severity: ticket
```

The same name across severities - `severity` is the discriminator - matches the way Prometheus alerts are typically structured.

## Run

```
opensloctl generate -f examples/multi-burn-slo -o output/
ls output/
# payment-availability-rules.yaml
```

## When to use this kind

Use `multi-burn-rate` when you want multiple burn-rate windows without the short/long AND pairing. If you need paired short+long windows, upgrade to [`multi-window-multi-burn-rate`](../../#multi-window-multi-burn-rate--sre-%C2%A76-recommended).
