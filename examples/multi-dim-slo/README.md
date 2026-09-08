# `multi-dim-slo` - One SLO expanded into many series by a label

Demonstrates the [`burn-rate`](../README.md#burn-rate--sre-%C2%A74) alerting strategy (SRE Workbook § 4) combined with the [multi-dimensional SLI annotations](../README.md#multi-dimensional-slis). One SLO becomes one series per value of the chosen Prometheus label.

## Strategy

A single `api-latency` SLO with target 0.999 (P99 < 500 ms) and a 30d rolling window. Two `burn-rate` AlertConditions - page at 14.4x over 5m, ticket at 3x over 2h - wrapped in their own AlertPolicies. No tiering, no AND pairs.

The multi-dim part: the SLO sets two annotations:

```yaml
metadata:
  annotations:
    multi-dimensional-sli.openslo.com/label: service_name
    multi-dimensional-sli.openslo.com/dimensions: "account,checkout,recommendation"
```

`service_name` is the dimension label that already exists on the underlying histogram series. The generator emits a `_unlabeled` layer of recording rules (carries `openslo_slo_name` only) and a `label_join` layer that joins the `service_name` value into `openslo_slo_name` with `-`, producing three series:

```
openslo_slo_current_burn_rate{openslo_slo_name="account-api-latency"}      = 0.2
openslo_slo_current_burn_rate{openslo_slo_name="checkout-api-latency"}     = 1.4
openslo_slo_current_burn_rate{openslo_slo_name="recommendation-api-latency"} = 4.7
```

Same SLO target, same alert policies, three independently firing series. Page on the dimension that crosses 14.4x; ignore the others.

## Files

```
multi-dim-slo/
├── README.md
├── service.yaml                        # `api-gateway` service
├── sli.yaml                            # thresholdMetric on http_request_duration_seconds_bucket
├── slo.yaml                            # annotations + indicatorRef + alertPolicies
├── alert-condition-page.yaml           # burn-rate, 14.4x, 5m
├── alert-condition-ticket.yaml         # burn-rate, 3x, 2h
├── alert-policy-page.yaml              # wraps the page condition
├── alert-policy-ticket.yaml            # wraps the ticket condition
├── notification-target-pagerduty.yaml
└── notification-target-slack.yaml
```

This example has no Makefile. Validate and generate via:

```bash
opensloctl validate -f examples/multi-dim-slo
opensloctl generate -f examples/multi-dim-slo -o output/
```

## What gets generated

One `api-latency-rules.yaml` per SLO. Inside:

- `openslo-sli-recordings-api-latency`: `_unlabeled` versions of `openslo_slo_info`, `openslo_slo_objective`, `openslo_sli_error_rate_5m`, `openslo_sli_error_rate_30m`, `openslo_sli_error_rate_1h`, `openslo_sli_event_rate_5m`, `openslo_slo_current_burn_rate`, `openslo_slo_period_burn_rate`, `openslo_slo_period_error_budget_remaining`, `openslo_slo_status`.
- A parallel `openslo-sli-recordings-api-latency-joined` group with `label_join` rules that promote the `service_name` value into `openslo_slo_name`.
- `openslo-alerts-api-latency`: one alert per severity (`ApiLatencyBurnRate`), each condition contributes one expression.

## When to use

Use multi-dim when:
- The underlying metric already splits by a label and you want shared target definitions across all series.
- Alert routing benefits from per-dimension firing rather than aggregate (per-caller-service paging, per-region escalation).
- You want per-dimension burn dashboards without writing one SLO per dimension.

Skip when:
- The chosen label is unbounded (`user_id`, raw trace IDs) - one recording rule per value burns Prometheus linearly.
- You only care about the aggregate across all series - a regular single-dim SLO is simpler.

## See also

- [Multi-dimensional SLIs](../README.md#multi-dimensional-slis) - registry entry, annotation semantics, runtime series shape.
- Burn-rate strategy details in [Alerting](../README.md#alerting) and [Burn-rate](../README.md#burn-rate--sre-%C2%A74).
