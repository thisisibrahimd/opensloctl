// Generate deploy/dashboards/openslo-list.json
// Variables: datasource (Prometheus picker), slo (multi + includeAll so the
// SLO source list drives both the table and the same-name fields in the
// table columns via $slo matcher in target expressions).
local g = import '../../tmp/grafonnet-vendor/vendor/github.com/grafana/grafonnet/gen/grafonnet-v13.0.0/main.libsonnet';

local var = g.dashboard.variable;
local panel = g.panel;
local promQ = g.query.prometheus;

local ds = { type: 'prometheus', uid: '${datasource}' };

local dsVar =
  var.datasource.new('datasource', 'prometheus')
  + var.datasource.generalOptions.withLabel('datasource')
  + var.datasource.selectionOptions.withIncludeAll(false)
  + var.datasource.selectionOptions.withMulti(false)
  + { allowCustomValue: false, pluginId: 'prometheus', refresh: 1 };

local sloVar =
  var.query.new('slo', 'label_values(openslo_slo_info, openslo_slo_name)')
  + var.query.withDatasource(type='prometheus', uid='${datasource}')
  + var.query.selectionOptions.withIncludeAll(true)
  + var.query.selectionOptions.withMulti(true)
  + var.query.refresh.onTime()
  + { allowCustomValue: false };

// StatusQ mapping: array form per current Grafana dashboards schema. Each
// entry carries a `value` field so Grafana treats the lookup as a value
// mapping rather than a label mapping. 0..3 enum drives the colors and
// text. Background fill is applied via custom.cellOptions + thresholds.
local statusMappings = [
  {
    type: 'value',
    options: {
      '0': { text: 'Healthy',  color: 'green',  index: 0, value: '0' },
      '1': { text: 'Burning',  color: 'yellow', index: 1, value: '1' },
      '2': { text: 'Critical', color: 'orange', index: 2, value: '2' },
      '3': { text: 'Breached', color: 'red',    index: 3, value: '3' },
    },
  },
];

local statusStatusQOverride = {
  matcher: { id: 'byName', options: 'Value #StatusQ' },
  properties: [
    { id: 'displayName', value: 'Status' },
    { id: 'mappings', value: statusMappings },
    { id: 'custom.cellOptions', value: { type: 'color-background', mode: 'basic', applyToRow: false } },
    { id: 'thresholds', value: {
      mode: 'absolute',
      steps: [
        { color: 'red',    value: null },
        { color: 'green',  value: 0 },
        { color: 'yellow', value: 1 },
        { color: 'orange', value: 2 },
        { color: 'red',    value: 3 },
      ],
    } },
  ],
};

local objectiveQOverride = {
  matcher: { id: 'byName', options: 'Value #ObjectiveQ' },
  properties: [
    { id: 'displayName', value: 'Objective %' },
    { id: 'unit', value: 'percent' },
    { id: 'decimals', value: 2 },
  ],
};

local periodSLIQOverride = {
  matcher: { id: 'byName', options: 'Value #PeriodSLIQ' },
  properties: [
    { id: 'displayName', value: 'Period SLI' },
    { id: 'unit', value: 'percent' },
    { id: 'decimals', value: 2 },
  ],
};

local budgetQOverride = {
  matcher: { id: 'byName', options: 'Value #BudgetQ' },
  properties: [
    { id: 'displayName', value: 'Budget Left %' },
    { id: 'unit', value: 'percent' },
    { id: 'custom.cellOptions', value: { type: 'color-background' } },
    { id: 'thresholds', value: {
      mode: 'absolute',
      steps: [
        { color: 'red',    value: null },
        { color: 'red',    value: 0 },
        { color: 'yellow', value: 15 },
        { color: 'green',  value: 50 },
      ],
    } },
    { id: 'decimals', value: 2 },
  ],
};

local timeHiddenOverride = {
  matcher: { id: 'byName', options: 'Time' },
  properties: [
    { id: 'custom.hidden', value: true },
  ],
};

local tablePanel =
  panel.table.new('Service Level Objectives')
  + {
    datasource: ds,
    targets: [
      promQ.new('${datasource}', 'openslo_slo_status{openslo_slo_name=~"$slo"}')
      + promQ.withRefId('StatusQ') + promQ.withInstant(true) + promQ.withFormat('table'),
      promQ.new('${datasource}', 'openslo_slo_objective{openslo_slo_name=~"$slo"} * 100')
      + promQ.withRefId('ObjectiveQ') + promQ.withInstant(true) + promQ.withFormat('table'),
      promQ.new('${datasource}', '(1 - openslo_sli_error_rate_30d{openslo_slo_name=~"$slo"}) * 100')
      + promQ.withRefId('PeriodSLIQ') + promQ.withInstant(true) + promQ.withFormat('table'),
      promQ.new('${datasource}', 'openslo_slo_period_error_budget_remaining{openslo_slo_name=~"$slo"} * 100')
      + promQ.withRefId('BudgetQ') + promQ.withInstant(true) + promQ.withFormat('table'),
    ],
    transformations: [
      { id: 'merge', options: {} },
      // includeByName is a positive-list filter stable across rule-template
      // refactors; drops __name__, chaos_flag, openslo_service_name,
      // openslo_spec_version, openslo_slo_description, and Time.
      { id: 'organize', options: {
        includeByName: {
          openslo_slo_name: true,
          'Value #StatusQ': true,
          'Value #ObjectiveQ': true,
          'Value #PeriodSLIQ': true,
          'Value #BudgetQ': true,
        },
        indexByName: {
          openslo_slo_name: 0,
          'Value #ObjectiveQ': 1,
          'Value #PeriodSLIQ': 2,
          'Value #StatusQ': 3,
          'Value #BudgetQ': 4,
        },
        renameByName: {
          openslo_slo_name: 'Service Level Objective',
        },
      } },
    ],
    options: {
      showHeader: true,
      sortBy: [{ desc: false, displayName: 'SLO name' }],
    },
    fieldConfig: {
      defaults: {
        custom: {
          align: 'left',
          cellOptions: { type: 'auto' },
        },
      },
      overrides: [
        statusStatusQOverride,
        objectiveQOverride,
        periodSLIQOverride,
        budgetQOverride,
        timeHiddenOverride,
      ],
    },
    gridPos: { x: 0, y: 0, w: 24, h: 24 },
  };

g.dashboard.new('OpenSLO - Manage SLOs')
+ g.dashboard.withUid('openslo-list')
+ g.dashboard.withDescription(
  'All SLOs loaded by opensloctl. Click any row to drill into the per-SLO detail dashboard.'
)
+ g.dashboard.withTags(['openslo'])
+ g.dashboard.time.withFrom('now-7d')
+ g.dashboard.time.withTo('now')
+ g.dashboard.withRefresh('30s')
+ g.dashboard.withVariables([dsVar, sloVar])
+ g.dashboard.withPanels([tablePanel])
