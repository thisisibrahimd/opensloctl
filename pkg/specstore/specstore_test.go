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
			name        string
			slis        []string
			indicatorRef *string
			wantErr     bool
			errContains string
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
			name         string
			alertPolicies []string
			policyRefs    []string
			wantErr      bool
			errContains  string
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
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(14.4), LookbackWindow: v1.NewDurationShorthand(5, v1.DurationShorthandUnitMinute)},
					},
				)
				specs.V1.AlertConditions["page-6x-30m"] = v1.NewAlertCondition(
					v1.Metadata{Name: "page-6x-30m"},
					v1.AlertConditionSpec{
						Severity: "page",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(6.0), LookbackWindow: v1.NewDurationShorthand(30, v1.DurationShorthandUnitMinute)},
					},
				)
				specs.V1.AlertConditions["ticket-3x-2h"] = v1.NewAlertCondition(
					v1.Metadata{Name: "ticket-3x-2h"},
					v1.AlertConditionSpec{
						Severity: "ticket",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(3.0), LookbackWindow: v1.NewDurationShorthand(2, v1.DurationShorthandUnitHour)},
					},
				)
				specs.V1.AlertConditions["ticket-1x-6h"] = v1.NewAlertCondition(
					v1.Metadata{Name: "ticket-1x-6h"},
					v1.AlertConditionSpec{
						Severity: "ticket",
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(1.0), LookbackWindow: v1.NewDurationShorthand(6, v1.DurationShorthandUnitHour)},
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
							Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
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
						Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
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
			wantErr     bool
			errContains string
		}{
			{
				name:    "burnrate valid",
				kind:    v1.AlertConditionKindBurnRate,
				wantErr: false,
			},
			{
				name:        "unknown kind rejected",
				kind:        v1.AlertConditionKind("latency"),
				wantErr:     true,
				errContains: `AlertCondition "test-cond" has kind "latency", only "burnrate" supported`,
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
						Condition: v1.AlertConditionType{Kind: tt.kind, Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
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

	t.Run("multiple errors collected", func(t *testing.T) {
		t.Parallel()

		specs := NewOpenSLOSpecs()
		specs.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})
		specs.V1.SLOs["test-slo"] = v1.NewSLO(
			v1.Metadata{Name: "test-slo"},
			v1.SLOSpec{
				Service:         "missing-svc",
				IndicatorRef:    ptr("missing-sli"),
				BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
				TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
				Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
			},
		)

		err := specs.ValidateRefs()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `SLO "test-slo" references Service "missing-svc" not found`)
		assert.Contains(t, err.Error(), `SLO "test-slo" references SLI "missing-sli" not found`)
	})

	t.Run("all refs resolve no error", func(t *testing.T) {
		t.Parallel()

		specs := NewOpenSLOSpecs()
		specs.V1.Services["svc"] = v1.NewService(v1.Metadata{Name: "svc"}, v1.ServiceSpec{})
		specs.V1.SLIs["sli"] = v1.NewSLI(
			v1.Metadata{Name: "sli"},
			v1.SLISpec{ThresholdMetric: &v1.SLIMetricSpec{MetricSource: v1.SLIMetricSource{Type: "Prometheus", Spec: map[string]any{"query": "up"}}}},
		)
		specs.V1.SLOs["test-slo"] = v1.NewSLO(
			v1.Metadata{Name: "test-slo"},
			v1.SLOSpec{
				Service:         "svc",
				IndicatorRef:    ptr("sli"),
				BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
				TimeWindow:      []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
				Objectives:      []v1.SLOObjective{{Target: ptr(0.999), Operator: v1.OperatorLTE, Value: ptr(500.0)}},
			},
		)

		err := specs.ValidateRefs()
		assert.NoError(t, err)
	})
}

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
