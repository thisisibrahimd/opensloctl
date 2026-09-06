package specstore

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/OpenSLO/go-sdk/pkg/openslo"
	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptr returns a pointer to v. Convenience helper for constructing optional
// fields in OpenSlo spec fixtures (e.g. Target, IndicatorRef).
func ptr[T any](v T) *T { return &v }

// TestNewOpenSLOSpecs verifies NewOpenSLOSpecs returns a non-nil store with
// every per-kind map initialized empty. Table-driven over each kind so
// failures name the specific map.
func TestNewOpenSLOSpecs(t *testing.T) {
	t.Parallel()

	lengths := []struct {
		name string
		size func(s *OpenSLOSpecs) int
	}{
		{"Services", func(s *OpenSLOSpecs) int { return len(s.V1.Services) }},
		{"SLOs", func(s *OpenSLOSpecs) int { return len(s.V1.SLOs) }},
		{"SLIs", func(s *OpenSLOSpecs) int { return len(s.V1.SLIs) }},
		{"DataSources", func(s *OpenSLOSpecs) int { return len(s.V1.DataSources) }},
		{"AlertPolices", func(s *OpenSLOSpecs) int { return len(s.V1.AlertPolices) }},
		{"AlertConditions", func(s *OpenSLOSpecs) int { return len(s.V1.AlertConditions) }},
		{"AlertNotificationTargets", func(s *OpenSLOSpecs) int { return len(s.V1.AlertNotificationTargets) }},
	}

	for _, l := range lengths {
		t.Run(l.name, func(t *testing.T) {
			t.Parallel()

			specs := NewOpenSLOSpecs()
			require.NotNil(t, specs)
			assert.Equal(t, 0, l.size(specs))
		})
	}
}

// TestStoreSpec covers three StoreSpec subtests:
//   - AllKinds: every supported kind (Service, SLO, SLI, DataSource,
//     AlertPolicy, AlertCondition, AlertNotificationTarget) round-trips
//     into its respective map.
//   - Duplicates: re-storing an object under an existing name returns an
//     error wrapping ERROR_SPEC_DUPLICATE.
//   - EdgeCases: storing multiple objects of different kinds works in a
//     single store.
func TestStoreSpec(t *testing.T) {
	t.Parallel()

	t.Run("AllKinds", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			object  openslo.Object
			expKind string
			expName string
		}{
			{
				name: "Service",
				object: v1.NewService(
					v1.Metadata{Name: "test-svc"},
					v1.ServiceSpec{Description: "test service"},
				),
				expKind: "Services",
				expName: "test-svc",
			},
			{
				name: "SLO",
				object: v1.NewSLO(
					v1.Metadata{Name: "test-slo"},
					v1.SLOSpec{
						Service:         "test-svc",
						IndicatorRef:    ptr("test-sli"),
						BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
						TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
						Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
					},
				),
				expKind: "SLOs",
				expName: "test-slo",
			},
			{
				name: "SLI",
				object: v1.NewSLI(
					v1.Metadata{Name: "test-sli"},
					v1.SLISpec{
						ThresholdMetric: &v1.SLIMetricSpec{
							MetricSource: v1.SLIMetricSource{
								Type: "Prometheus",
								Spec: map[string]any{"query": "up"},
							},
						},
					},
				),
				expKind: "SLIs",
				expName: "test-sli",
			},
			{
				name: "DataSource",
				object: v1.NewDataSource(
					v1.Metadata{Name: "test-ds"},
					v1.DataSourceSpec{
						Type:              "Prometheus",
						ConnectionDetails: json.RawMessage(`{"url":"http://prom:9090"}`),
					},
				),
				expKind: "DataSources",
				expName: "test-ds",
			},
			{
				name: "AlertPolicy",
				object: v1.NewAlertPolicy(
					v1.Metadata{Name: "test-ap"},
					v1.AlertPolicySpec{
						AlertWhenBreaching: true,
						Conditions: []v1.AlertPolicyCondition{
							{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: "test-ac"}},
						},
						NotificationTargets: []v1.AlertPolicyNotificationTarget{
							{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: "test-ant"}},
						},
					},
				),
				expKind: "AlertPolices",
				expName: "test-ap",
			},
			{
				name: "AlertCondition",
				object: v1.NewAlertCondition(
					v1.Metadata{Name: "test-ac"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{
							Kind:           v1.AlertConditionKind("multi-window-multi-burn-rate"),
							Operator:       v1.OperatorLTE,
							Threshold:      ptr(2.0),
							LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour),
						},
					},
				),
				expKind: "AlertConditions",
				expName: "test-ac",
			},
			{
				name: "AlertNotificationTarget",
				object: v1.NewAlertNotificationTarget(
					v1.Metadata{Name: "test-ant"},
					v1.AlertNotificationTargetSpec{
						Target:      "pagerduty",
						Description: "test target",
					},
				),
				expKind: "AlertNotificationTargets",
				expName: "test-ant",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				err := specs.StoreSpec(tt.object)
				require.NoError(t, err)

				switch tt.expKind {
				case "Services":
					assert.Contains(t, specs.V1.Services, tt.expName)
				case "SLOs":
					assert.Contains(t, specs.V1.SLOs, tt.expName)
				case "SLIs":
					assert.Contains(t, specs.V1.SLIs, tt.expName)
				case "DataSources":
					assert.Contains(t, specs.V1.DataSources, tt.expName)
				case "AlertPolices":
					assert.Contains(t, specs.V1.AlertPolices, tt.expName)
				case "AlertConditions":
					assert.Contains(t, specs.V1.AlertConditions, tt.expName)
				case "AlertNotificationTargets":
					assert.Contains(t, specs.V1.AlertNotificationTargets, tt.expName)
				}
			})
		}
	})

	t.Run("Duplicates", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			first     openslo.Object
			second    openslo.Object
			wantError bool
		}{
			{
				name:   "Service",
				first:  v1.NewService(v1.Metadata{Name: "dup-svc"}, v1.ServiceSpec{}),
				second: v1.NewService(v1.Metadata{Name: "dup-svc"}, v1.ServiceSpec{Description: "second"}),
			},
			{
				name: "SLO",
				first: v1.NewSLO(v1.Metadata{Name: "dup-slo"}, v1.SLOSpec{
					Service: "svc", IndicatorRef: ptr("sli"), BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow: []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives: []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				}),
				second: v1.NewSLO(v1.Metadata{Name: "dup-slo"}, v1.SLOSpec{
					Service: "svc", IndicatorRef: ptr("sli"), BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow: []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives: []v1.SLOObjective{{Target: ptr(0.99), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				}),
			},
			{
				name: "SLI",
				first: v1.NewSLI(v1.Metadata{Name: "dup-sli"}, v1.SLISpec{
					ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}},
				}),
				second: v1.NewSLI(v1.Metadata{Name: "dup-sli"}, v1.SLISpec{
					ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}},
				}),
			},
			{
				name:   "DataSource",
				first:  v1.NewDataSource(v1.Metadata{Name: "dup-ds"}, v1.DataSourceSpec{Type: "Prometheus", ConnectionDetails: json.RawMessage(`{}`)}),
				second: v1.NewDataSource(v1.Metadata{Name: "dup-ds"}, v1.DataSourceSpec{Type: "Prometheus", ConnectionDetails: json.RawMessage(`{}`)}),
			},
			{
				name: "AlertPolicy",
				first: v1.NewAlertPolicy(v1.Metadata{Name: "dup-ap"}, v1.AlertPolicySpec{
					AlertWhenBreaching: true,
					Conditions:         []v1.AlertPolicyCondition{{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: "ac"}}},
					NotificationTargets: []v1.AlertPolicyNotificationTarget{{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: "ant"}}},
				}),
				second: v1.NewAlertPolicy(v1.Metadata{Name: "dup-ap"}, v1.AlertPolicySpec{
					AlertWhenBreaching: true,
					Conditions:         []v1.AlertPolicyCondition{{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: "ac"}}},
					NotificationTargets: []v1.AlertPolicyNotificationTarget{{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: "ant"}}},
				}),
			},
			{
				name: "AlertCondition",
				first: v1.NewAlertCondition(v1.Metadata{Name: "dup-ac"}, v1.AlertConditionSpec{
					Severity: "page",
					Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
				}),
				second: v1.NewAlertCondition(v1.Metadata{Name: "dup-ac"}, v1.AlertConditionSpec{
					Severity: "warn",
					Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
				}),
			},
			{
				name:   "AlertNotificationTarget",
				first:  v1.NewAlertNotificationTarget(v1.Metadata{Name: "dup-ant"}, v1.AlertNotificationTargetSpec{Target: "pagerduty"}),
				second: v1.NewAlertNotificationTarget(v1.Metadata{Name: "dup-ant"}, v1.AlertNotificationTargetSpec{Target: "slack"}),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				require.NoError(t, specs.StoreSpec(tt.first))

				err := specs.StoreSpec(tt.second)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "duplicate spec")
			})
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			objects     []openslo.Object
			checkFunc   func(t *testing.T, specs *OpenSLOSpecs)
			wantErr     []bool
			errContains []string
		}{
			{
				name: "multiple kinds",
				objects: []openslo.Object{
					v1.NewService(v1.Metadata{Name: "multi-svc"}, v1.ServiceSpec{Description: "multi test service"}),
					v1.NewSLI(v1.Metadata{Name: "multi-sli"}, v1.SLISpec{
						ThresholdMetric: &v1.SLIMetricSpec{
							MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
						},
					}),
					v1.NewSLO(v1.Metadata{Name: "multi-slo"}, v1.SLOSpec{
						Service: "multi-svc", IndicatorRef: ptr("multi-sli"), BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
						TimeWindow: []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
						Objectives: []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
					}),
				},
				wantErr: []bool{false, false, false},
				checkFunc: func(t *testing.T, specs *OpenSLOSpecs) {
					t.Helper()
					assert.Contains(t, specs.V1.Services, "multi-svc")
					assert.Contains(t, specs.V1.SLIs, "multi-sli")
					assert.Contains(t, specs.V1.SLOs, "multi-slo")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for i, obj := range tt.objects {
					err := specs.StoreSpec(obj)
					if len(tt.wantErr) > i {
						if tt.wantErr[i] {
							assert.Error(t, err)
							if len(tt.errContains) > i {
								assert.Contains(t, err.Error(), tt.errContains[i])
							}
						} else {
							assert.NoError(t, err)
						}
					}
				}
				if tt.checkFunc != nil {
					tt.checkFunc(t, specs)
				}
			})
		}
	})
}

// TestStoreSpec_ValidationErrors verifies SDK validation runs before
// opensloctl-specific checks. Each case constructs an object with/without
// required SDK fields and asserts StoreSpec returns or skips errors.
func TestStoreSpec_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		object    openslo.Object
		wantError bool
	}{
		{
			name: "valid Service",
			object: v1.NewService(
				v1.Metadata{Name: "valid-svc"},
				v1.ServiceSpec{Description: "valid"},
			),
			wantError: false,
		},
		{
			name: "valid SLO",
			object: v1.NewSLO(
				v1.Metadata{Name: "valid-slo"},
				v1.SLOSpec{
					Service: "svc",
					Indicator: &v1.SLOIndicatorInline{
						Metadata: v1.Metadata{Name: "valid-sli"},
						Spec: v1.SLISpec{
							ThresholdMetric: &v1.SLIMetricSpec{
								MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
							},
						},
					},
					BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				},
			),
			wantError: false,
		},
		{
			name: "valid SLI",
			object: v1.NewSLI(
				v1.Metadata{Name: "valid-sli"},
				v1.SLISpec{
					ThresholdMetric: &v1.SLIMetricSpec{
						MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
					},
				},
			),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			specs := NewOpenSLOSpecs()
			err := specs.StoreSpec(tt.object)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStoreSpec_TargetRequired verifies the opensloctl-specific post-check:
// every SLO objective must declare either target (0-1 scale) or
// targetPercent (0-100 scale). Missing both surfaces a "one of
// [target, targetPercent] properties must be set" error from the SDK
// because the SDK actually enforces this despite the OpenSlo spec
// marking it optional.
func TestStoreSpec_TargetRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		object      openslo.Object
		wantError   bool
		errContains string
	}{
		{
			name: "valid with target",
			object: v1.NewSLO(
				v1.Metadata{Name: "valid-target"},
				v1.SLOSpec{
					Service: "svc",
					Indicator: &v1.SLOIndicatorInline{
						Metadata: v1.Metadata{Name: "sli"},
						Spec: v1.SLISpec{
							ThresholdMetric: &v1.SLIMetricSpec{
								MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
							},
						},
					},
					BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives:      []v1.SLOObjective{{Target: ptr(0.99), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				},
			),
			wantError: false,
		},
		{
			name: "valid with targetPercent",
			object: v1.NewSLO(
				v1.Metadata{Name: "valid-percent"},
				v1.SLOSpec{
					Service: "svc",
					Indicator: &v1.SLOIndicatorInline{
						Metadata: v1.Metadata{Name: "sli"},
						Spec: v1.SLISpec{
							ThresholdMetric: &v1.SLIMetricSpec{
								MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
							},
						},
					},
					BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives:      []v1.SLOObjective{{TargetPercent: ptr(99.0), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				},
			),
			wantError: false,
		},
		{
			name: "missing target rejected",
			object: v1.NewSLO(
				v1.Metadata{Name: "missing-target"},
				v1.SLOSpec{
					Service: "svc",
					Indicator: &v1.SLOIndicatorInline{
						Metadata: v1.Metadata{Name: "sli"},
						Spec: v1.SLISpec{
							ThresholdMetric: &v1.SLIMetricSpec{
								MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}},
							},
						},
					},
					BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives:      []v1.SLOObjective{{Operator: v1.OperatorLTE, Value: ptr(500.0)}},
				},
			),
			wantError:   true,
			errContains: `one of [target, targetPercent] properties must be set`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			specs := NewOpenSLOSpecs()
			err := specs.StoreSpec(tt.object)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateRefs covers cross-reference resolution and burn-rate rule-set
// completeness. Every subtest is table-driven:
//
//   - SLO → Service            resolve / unresolve
//   - SLO → SLI (indicatorRef) resolve / unresolve / nil-ref skip
//   - SLO → AlertPolicy        resolve / unresolve
//   - AlertPolicy → Condition  resolve / unresolve
//   - AlertPolicy → Notif      resolve / unresolve
//   - SLI → DataSource         resolve / unresolve / empty-ref skip
//   - AlertCondition kind      burnrate ok / latency rejected
//   - SLO graph                multi-error chain / all-resolve baseline
//
// The "multiple errors collected" case (last subtest's first row) confirms
// a single call surfaces every missing ref independently.
func TestValidateRefs(t *testing.T) {
	t.Parallel()

	t.Run("SLO to Service", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			services    []string
			sloService  string
			wantErr     bool
			errContains string
		}{
			{
				name:       "resolved",
				services:   []string{"my-svc"},
				sloService: "my-svc",
				wantErr:    false,
			},
			{
				name:        "unresolved",
				services:    []string{"other-svc"},
				sloService:  "missing-svc",
				wantErr:     true,
				errContains: `SLO "test-slo" references Service "missing-svc" not found`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, svc := range tt.services {
					specs.V1.Services[svc] = v1.NewService(v1.Metadata{Name: svc}, v1.ServiceSpec{})
				}
				specs.V1.SLOs["test-slo"] = v1.NewSLO(
					v1.Metadata{Name: "test-slo"},
					v1.SLOSpec{
						Service: tt.sloService,
						Indicator: &v1.SLOIndicatorInline{
							Metadata: v1.Metadata{Name: "sli"},
							Spec: v1.SLISpec{
								ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}},
							},
						},
						BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
						TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
						Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
					},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("SLO to SLI indicatorRef", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			slis         []string
			indicatorRef *string
			wantErr      bool
			errContains  string
		}{
			{
				name:         "resolved",
				slis:         []string{"my-sli"},
				indicatorRef: ptr("my-sli"),
				wantErr:      false,
			},
			{
				name:         "unresolved",
				slis:         []string{"other-sli"},
				indicatorRef: ptr("missing-sli"),
				wantErr:      true,
				errContains:  `SLO "test-slo" references SLI "missing-sli" not found`,
			},
			{
				name:         "nil ref skipped",
				slis:         []string{},
				indicatorRef: nil,
				wantErr:      false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, sli := range tt.slis {
					specs.V1.SLIs[sli] = v1.NewSLI(
						v1.Metadata{Name: sli},
						v1.SLISpec{
							ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}},
						},
					)
				}
				specs.V1.SLOs["test-slo"] = v1.NewSLO(
					v1.Metadata{Name: "test-slo"},
					v1.SLOSpec{
						Service:         "svc",
						IndicatorRef:    tt.indicatorRef,
						BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
						TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
						Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
					},
				)
				specs.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("SLO to AlertPolicy", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name          string
			alertPolicies []string
			policyRefs    []string
			wantErr       bool
			errContains   string
		}{
			{
				name:          "resolved",
				alertPolicies: []string{"page-14-4x-5m", "page-6x-30m", "ticket-3x-2h", "ticket-1x-6h"},
				policyRefs:    []string{"page-14-4x-5m", "page-6x-30m", "ticket-3x-2h", "ticket-1x-6h"},
				wantErr:       false,
			},
			{
				name:          "unresolved",
				alertPolicies: []string{"other-policy"},
				policyRefs:    []string{"missing-policy"},
				wantErr:       true,
				errContains:   `SLO "test-slo" references AlertPolicy "missing-policy" not found`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, pol := range tt.alertPolicies {
					specs.V1.AlertPolices[pol] = v1.NewAlertPolicy(
						v1.Metadata{Name: pol},
						v1.AlertPolicySpec{
							AlertWhenBreaching: true,
							Conditions:         []v1.AlertPolicyCondition{{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: pol}}},
							NotificationTargets: []v1.AlertPolicyNotificationTarget{{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: "nt"}}},
						},
					)
				}
				specs.V1.AlertConditions["page-14-4x-5m"] = v1.NewAlertCondition(
					v1.Metadata{Name: "page-14-4x-5m"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(14.4), LookbackWindow: v1.NewDurationShorthand(5, v1.DurationShorthandUnitMinute)},
					},
				)
				specs.V1.AlertConditions["page-6x-30m"] = v1.NewAlertCondition(
					v1.Metadata{Name: "page-6x-30m"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(6.0), LookbackWindow: v1.NewDurationShorthand(30, v1.DurationShorthandUnitMinute)},
					},
				)
				specs.V1.AlertConditions["ticket-3x-2h"] = v1.NewAlertCondition(
					v1.Metadata{Name: "ticket-3x-2h"},
					v1.AlertConditionSpec{
						Severity: "ticket",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(3.0), LookbackWindow: v1.NewDurationShorthand(2, v1.DurationShorthandUnitHour)},
					},
				)
				specs.V1.AlertConditions["ticket-1x-6h"] = v1.NewAlertCondition(
					v1.Metadata{Name: "ticket-1x-6h"},
					v1.AlertConditionSpec{
						Severity: "ticket",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(1.0), LookbackWindow: v1.NewDurationShorthand(6, v1.DurationShorthandUnitHour)},
					},
				)
				specs.V1.AlertNotificationTargets["nt"] = v1.NewAlertNotificationTarget(
					v1.Metadata{Name: "nt"},
					v1.AlertNotificationTargetSpec{Target: "pagerduty"},
				)
				specs.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})
				var refs []v1.SLOAlertPolicy
				for _, ref := range tt.policyRefs {
					refs = append(refs, v1.SLOAlertPolicy{SLOAlertPolicyRef: &v1.SLOAlertPolicyRef{AlertPolicyRef: ref}})
				}
				specs.V1.SLOs["test-slo"] = v1.NewSLO(
					v1.Metadata{Name: "test-slo"},
					v1.SLOSpec{
						Service: "svc",
						Indicator: &v1.SLOIndicatorInline{
							Metadata: v1.Metadata{Name: "sli"},
							Spec: v1.SLISpec{
								ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}},
							},
						},
						BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
						TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
						Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
						AlertPolicies:   refs,
					},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("AlertPolicy to AlertCondition", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			conditions  []string
			condRefs    []string
			wantErr     bool
			errContains string
		}{
			{
				name:       "resolved",
				conditions: []string{"my-cond"},
				condRefs:   []string{"my-cond"},
				wantErr:    false,
			},
			{
				name:        "unresolved",
				conditions:  []string{"other-cond"},
				condRefs:    []string{"missing-cond"},
				wantErr:     true,
				errContains: `AlertPolicy "test-pol" references AlertCondition "missing-cond" not found`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, cond := range tt.conditions {
					specs.V1.AlertConditions[cond] = v1.NewAlertCondition(
						v1.Metadata{Name: cond},
						v1.AlertConditionSpec{
							Severity: "page",
							Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
						},
					)
				}
				var conds []v1.AlertPolicyCondition
				for _, ref := range tt.condRefs {
					conds = append(conds, v1.AlertPolicyCondition{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: ref}})
				}
				specs.V1.AlertPolices["test-pol"] = v1.NewAlertPolicy(
					v1.Metadata{Name: "test-pol"},
					v1.AlertPolicySpec{
						AlertWhenBreaching: true,
						Conditions:         conds,
						NotificationTargets: []v1.AlertPolicyNotificationTarget{{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: "nt"}}},
					},
				)
				specs.V1.AlertNotificationTargets["nt"] = v1.NewAlertNotificationTarget(
					v1.Metadata{Name: "nt"},
					v1.AlertNotificationTargetSpec{Target: "pagerduty"},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("AlertPolicy to AlertNotificationTarget", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			targets     []string
			targetRefs  []string
			wantErr     bool
			errContains string
		}{
			{
				name:       "resolved",
				targets:    []string{"my-target"},
				targetRefs: []string{"my-target"},
				wantErr:    false,
			},
			{
				name:        "unresolved",
				targets:     []string{"other-target"},
				targetRefs:  []string{"missing-target"},
				wantErr:     true,
				errContains: `AlertPolicy "test-pol" references AlertNotificationTarget "missing-target" not found`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, nt := range tt.targets {
					specs.V1.AlertNotificationTargets[nt] = v1.NewAlertNotificationTarget(
						v1.Metadata{Name: nt},
						v1.AlertNotificationTargetSpec{Target: "pagerduty"},
					)
				}
				var nts []v1.AlertPolicyNotificationTarget
				for _, ref := range tt.targetRefs {
					nts = append(nts, v1.AlertPolicyNotificationTarget{AlertPolicyNotificationTargetRef: &v1.AlertPolicyNotificationTargetRef{TargetRef: ref}})
				}
				specs.V1.AlertPolices["test-pol"] = v1.NewAlertPolicy(
					v1.Metadata{Name: "test-pol"},
					v1.AlertPolicySpec{
						AlertWhenBreaching: true,
						Conditions:         []v1.AlertPolicyCondition{{AlertPolicyConditionRef: &v1.AlertPolicyConditionRef{ConditionRef: "cond"}}},
						NotificationTargets: nts,
					},
				)
				specs.V1.AlertConditions["cond"] = v1.NewAlertCondition(
					v1.Metadata{Name: "cond"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKind("multi-window-multi-burn-rate"), Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
					},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("SLI to DataSource", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			dataSources []string
			sourceRef   string
			wantErr     bool
			errContains string
		}{
			{
				name:        "resolved",
				dataSources: []string{"my-ds"},
				sourceRef:   "my-ds",
				wantErr:     false,
			},
			{
				name:        "unresolved",
				dataSources: []string{"other-ds"},
				sourceRef:   "missing-ds",
				wantErr:     true,
				errContains: `SLI "test-sli" thresholdMetric references DataSource "missing-ds" not found`,
			},
			{
				name:      "empty ref skipped",
				sourceRef: "",
				wantErr:   false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				for _, ds := range tt.dataSources {
					specs.V1.DataSources[ds] = v1.NewDataSource(
						v1.Metadata{Name: ds},
						v1.DataSourceSpec{Type: "Prometheus", ConnectionDetails: json.RawMessage(`{}`)},
					)
				}
				metricSource := v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}
				if tt.sourceRef != "" {
					metricSource.MetricSourceRef = tt.sourceRef
				}
				specs.V1.SLIs["test-sli"] = v1.NewSLI(
					v1.Metadata{Name: "test-sli"},
					v1.SLISpec{
						ThresholdMetric: &v1.SLIMetricSpec{MetricSource: metricSource},
					},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("AlertCondition kind validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			kind        v1.AlertConditionKind
			threshold   float64
			wantErr     bool
			errContains string
		}{
			{
				name:      "burnrate kind rejected (legacy not accepted)",
				kind:      v1.AlertConditionKindBurnRate,
				threshold: 14.4,
				wantErr:   true,
				errContains: `AlertCondition "test-cond" has kind "burnrate", supported kinds`,
			},
			{
				name:      "multi-window-multi-burn-rate valid",
				kind:      v1.AlertConditionKind("multi-window-multi-burn-rate"),
				threshold: 14.4,
				wantErr:   false,
			},
			{
				name:      "burn-rate valid (single threshold)",
				kind:      v1.AlertConditionKind("burn-rate"),
				threshold: 14.4,
				wantErr:   false,
			},
			{
				name:      "error-rate valid (absolute threshold in (0,1])",
				kind:      v1.AlertConditionKind("error-rate"),
				threshold: 0.001,
				wantErr:   false,
			},
			{
				name:      "error-rate threshold > 1 rejected",
				kind:      v1.AlertConditionKind("error-rate"),
				threshold: 2.0,
				wantErr:   true,
				errContains: `error-rate threshold must be in (0, 1]`,
			},
			{
				name:      "unknown kind rejected",
				kind:      v1.AlertConditionKind("latency"),
				threshold: 14.4,
				wantErr:   true,
				errContains: `AlertCondition "test-cond" has kind "latency", supported kinds`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				specs.V1.AlertConditions["test-cond"] = v1.NewAlertCondition(
					v1.Metadata{Name: "test-cond"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{Kind: tt.kind, Operator: v1.OperatorLTE, Threshold: ptr(tt.threshold), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
					},
				)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errContains)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("SLO graph", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			setup       func(*OpenSLOSpecs)
			wantErr     bool
			errContains []string
		}{
			{
				name: "multiple errors collected",
				setup: func(s *OpenSLOSpecs) {
					s.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})
					s.V1.SLOs["test-slo"] = v1.NewSLO(
						v1.Metadata{Name: "test-slo"},
						v1.SLOSpec{
							Service:         "missing-svc",
							IndicatorRef:    ptr("missing-sli"),
							BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
							TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
							Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
						},
					)
				},
				wantErr: true,
				errContains: []string{
					`SLO "test-slo" references Service "missing-svc" not found`,
					`SLO "test-slo" references SLI "missing-sli" not found`,
				},
			},
			{
				name: "all refs resolve no error",
				setup: func(s *OpenSLOSpecs) {
					s.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})
					s.V1.SLIs["sli"] = v1.NewSLI(
						v1.Metadata{Name: "sli"},
						v1.SLISpec{ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}}},
					)
					s.V1.SLOs["test-slo"] = v1.NewSLO(
						v1.Metadata{Name: "test-slo"},
						v1.SLOSpec{
							Service:         "svc",
							IndicatorRef:    ptr("sli"),
							BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
							TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
							Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
						},
					)
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				specs := NewOpenSLOSpecs()
				tt.setup(specs)

				err := specs.ValidateRefs()
				if tt.wantErr {
					require.Error(t, err)
					for _, sub := range tt.errContains {
						assert.Contains(t, err.Error(), sub)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// TestGetSpecs covers the file-loading surface of GetSpecs:
// single files, multi-document YAML, recursive directory scan, blank and
// missing inputs, invalid/non-OpenSlo YAML silently skipped, and
// deduplication when the same file is passed twice.
func TestGetSpecs(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("testdata")

	tests := []struct {
		name            string
		files           []string
		recursive       bool
		wantErr         bool
		errContains     string
		checkFunc       func(t *testing.T, specs *OpenSLOSpecs)
		wantService     string
		wantSLO         string
		wantSLI         string
		wantDataSource  string
		wantAlertPolicy string
	}{
		{
			name:        "single service file",
			files:       []string{filepath.Join(testdata, "service.yaml")},
			wantService: "test-service",
		},
		{
			name:    "single slo file",
			files:   []string{filepath.Join(testdata, "slo.yaml"), filepath.Join(testdata, "service.yaml")},
			wantSLO: "test-slo",
		},
		{
			name:        "multiple individual files",
			files:       []string{filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "sli.yaml")},
			wantService: "test-service",
			wantSLI:     "test-sli",
		},
		{
			name:        "directory non-recursive",
			files:       []string{testdata},
			recursive:   false,
			wantService: "test-service",
		},
		{
			name:        "directory recursive",
			files:       []string{testdata},
			recursive:   true,
			wantService: "test-service",
		},
		{
			name:        "multi-document yaml",
			files:       []string{filepath.Join(testdata, "multi-doc.yaml")},
			wantService: "svc-a",
			wantSLO:     "slo-a",
		},
		{
			name:           "populates all kinds",
			files:          []string{testdata},
			recursive:      true,
			wantService:    "test-service",
			wantSLO:        "test-slo",
			wantSLI:        "test-sli",
			wantDataSource: "test-datasource",
			wantAlertPolicy: "test-alert-policy",
		},
		{
			name:        "duplicate files deduplicated",
			files:       []string{filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "service.yaml")},
			wantService: "test-service",
			checkFunc: func(t *testing.T, specs *OpenSLOSpecs) {
				t.Helper()
				assert.Len(t, specs.V1.Services, 1)
			},
		},
		{
			name:        "empty filenames",
			files:       []string{},
			wantErr:     true,
			errContains: "error detecting files",
		},
		{
			name:        "missing file",
			files:       []string{"nonexistent.yaml"},
			wantErr:     true,
			errContains: "error detecting files",
		},
		{
			name:      "invalid yaml skipped",
			files:     []string{filepath.Join(testdata, "invalid.yaml")},
			checkFunc: func(t *testing.T, specs *OpenSLOSpecs) {
				t.Helper()
				assert.Empty(t, specs.V1.Services)
			},
		},
		{
			name:      "non-openslo yaml skipped",
			files:     []string{filepath.Join(testdata, "non-openslo.yaml")},
			checkFunc: func(t *testing.T, specs *OpenSLOSpecs) {
				t.Helper()
				assert.Empty(t, specs.V1.Services)
			},
		},
		{
			name:        "mixed valid and invalid",
			files:       []string{filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "invalid.yaml")},
			wantService: "test-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			specs, err := GetSpecs(tt.files, tt.recursive)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, specs)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, specs)

			if tt.wantService != "" {
				assert.Contains(t, specs.V1.Services, tt.wantService)
			}
			if tt.wantSLO != "" {
				assert.Contains(t, specs.V1.SLOs, tt.wantSLO)
			}
			if tt.wantSLI != "" {
				assert.Contains(t, specs.V1.SLIs, tt.wantSLI)
			}
			if tt.wantDataSource != "" {
				assert.Contains(t, specs.V1.DataSources, tt.wantDataSource)
			}
			if tt.wantAlertPolicy != "" {
				assert.Contains(t, specs.V1.AlertPolices, tt.wantAlertPolicy)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, specs)
			}
		})
	}
}

// TestGetSpecs_RefValidation confirms GetSpecs runs ValidateRefs and
// surfaces ref errors via xerrors.Join. Specifically tests:
//   - Recursive load of the testdata directory resolves every cross-ref
//   - A slo-with-refs file loaded alongside the matching service/sli/alert
//     objects passes validation
func TestGetSpecs_RefValidation(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("testdata")

	tests := []struct {
		name        string
		files       []string
		wantErr     bool
		errContains string
	}{
		{
			name:    "all refs resolve",
			files:   []string{testdata},
			wantErr: false,
		},
		{
			name:    "slo with refs to existing objects",
			files:   []string{filepath.Join(testdata, "slo-with-refs.yaml"), filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "sli.yaml"), filepath.Join(testdata, "alert-policy.yaml"), filepath.Join(testdata, "alert-condition.yaml"), filepath.Join(testdata, "notification-target.yaml")},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			specs, err := GetSpecs(tt.files, true)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, specs)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, specs)
			}
		})
	}
}

// TestLoadSpecs covers internal loadSpec/loadSpecs:
//   - LoadSpec returns the correct number of objects, kind, and name for
//     valid YAML; emits parse/read errors with stable messages for invalid
//     and missing files.
//   - LoadSpecs collects objects across multiple files and skips files that
//     fail to parse (store-level validation runs separately).
func TestLoadSpecs(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("testdata")

	t.Run("LoadSpec", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			filename    string
			wantErr     bool
			errContains string
			wantCount   int
			wantKind    openslo.Kind
			wantName    string
		}{
			{
				name:        "valid service yaml",
				filename:    filepath.Join(testdata, "service.yaml"),
				wantCount:   1,
				wantKind:    openslo.KindService,
				wantName:    "test-service",
			},
			{
				name:        "multi-document yaml",
				filename:    filepath.Join(testdata, "multi-doc.yaml"),
				wantCount:   2,
			},
			{
				name:        "invalid yaml",
				filename:    filepath.Join(testdata, "invalid.yaml"),
				wantErr:     true,
				errContains: "error parsing spec",
			},
			{
				name:        "missing file",
				filename:    "nonexistent-file.yaml",
				wantErr:     true,
				errContains: "error reading file",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				objects, err := loadSpec(tt.filename)

				if tt.wantErr {
					assert.Error(t, err)
					if tt.errContains != "" {
						assert.Contains(t, err.Error(), tt.errContains)
					}
					return
				}

				require.NoError(t, err)
				assert.Len(t, objects, tt.wantCount)
				if tt.wantKind != "" && len(objects) > 0 {
					assert.Equal(t, tt.wantKind, objects[0].GetKind())
				}
				if tt.wantName != "" && len(objects) > 0 {
					assert.Equal(t, tt.wantName, objects[0].GetName())
				}
			})
		}
	})

	t.Run("LoadSpecs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			files     []string
			wantErr   bool
			wantCount int
		}{
			{
				name:      "multiple files",
				files:     []string{filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "sli.yaml")},
				wantCount: 2,
			},
			{
				name:      "skip errors in middle",
				files:     []string{filepath.Join(testdata, "service.yaml"), filepath.Join(testdata, "invalid.yaml"), filepath.Join(testdata, "sli.yaml")},
				wantCount: 2,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				objects, err := loadSpecs(tt.files)
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Len(t, objects, tt.wantCount)
			})
		}
	})
}

// BenchmarkStoreSpec measures per-iteration cost of constructing a fresh
// store and inserting a single Service object.
func BenchmarkStoreSpec(b *testing.B) {
	svc := v1.NewService(
		v1.Metadata{Name: "bench-svc"},
		v1.ServiceSpec{Description: "benchmark service"},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		specs := NewOpenSLOSpecs()
		_ = specs.StoreSpec(svc)
	}
}

// BenchmarkGetSpecs measures end-to-end GetSpecs cost over the testdata/
// directory (one shallow load per iteration).
func BenchmarkGetSpecs(b *testing.B) {
	testdata := filepath.Join("testdata")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetSpecs([]string{testdata}, false)
	}
}
