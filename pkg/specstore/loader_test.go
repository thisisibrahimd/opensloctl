package specstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenSLO/go-sdk/pkg/openslo"
	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestNewOpenSLOSpecs(t *testing.T) {
	t.Parallel()

	specs := NewOpenSLOSpecs()

	require.NotNil(t, specs)
	require.NotNil(t, specs.V1.Services)
	require.NotNil(t, specs.V1.SLOs)
	require.NotNil(t, specs.V1.SLIs)
	require.NotNil(t, specs.V1.DataSources)
	require.NotNil(t, specs.V1.AlertPolices)
	require.NotNil(t, specs.V1.AlertConditions)
	require.NotNil(t, specs.V1.AlertNotificationTargets)

	assert.Empty(t, specs.V1.Services)
	assert.Empty(t, specs.V1.SLOs)
	assert.Empty(t, specs.V1.SLIs)
	assert.Empty(t, specs.V1.DataSources)
	assert.Empty(t, specs.V1.AlertPolices)
	assert.Empty(t, specs.V1.AlertConditions)
	assert.Empty(t, specs.V1.AlertNotificationTargets)
}

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
						Objectives:      []v1.SLOObjective{{Target: ptr(0.999)}},
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
							Kind:           v1.AlertConditionKindBurnRate,
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
					Objectives: []v1.SLOObjective{{Target: ptr(0.999)}},
				}),
				second: v1.NewSLO(v1.Metadata{Name: "dup-slo"}, v1.SLOSpec{
					Service: "svc", IndicatorRef: ptr("sli"), BudgetingMethod: v1.SLOBudgetingMethodOccurrences,
					TimeWindow: []v1.SLOTimeWindow{{Duration: v1.NewDurationShorthand(30, v1.DurationShorthandUnitDay), IsRolling: true}},
					Objectives: []v1.SLOObjective{{Target: ptr(0.99)}},
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
					Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
				}),
				second: v1.NewAlertCondition(v1.Metadata{Name: "dup-ac"}, v1.AlertConditionSpec{
					Severity: "warn",
					Condition: v1.AlertConditionType{Kind: v1.AlertConditionKindBurnRate, Operator: v1.OperatorLTE, Threshold: ptr(2.0), LookbackWindow: v1.NewDurationShorthand(1, v1.DurationShorthandUnitHour)},
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
						Objectives: []v1.SLOObjective{{Target: ptr(0.999)}},
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
			files:   []string{filepath.Join(testdata, "slo.yaml")},
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
			name:        "populates all kinds",
			files:       []string{testdata},
			recursive:   true,
			wantService: "test-service",
			wantSLO:     "test-slo",
			wantSLI:     "test-sli",
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
			name:    "empty filenames",
			files:   []string{},
			wantErr: true,
			errContains: "error detecting files",
		},
		{
			name:    "missing file",
			files:   []string{"nonexistent.yaml"},
			wantErr: true,
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

func BenchmarkGetSpecs(b *testing.B) {
	testdata := filepath.Join("testdata")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetSpecs([]string{testdata}, false)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
