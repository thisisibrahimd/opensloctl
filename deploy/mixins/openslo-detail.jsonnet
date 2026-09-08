// Per-SLO detail dashboard.
// Variables: datasource, slo (set via URL when drilling from the list),
// description + target (hidden, derived from openslo_slo_info /
// openslo_slo_objective) -- text panel inlines them under the SLO name.
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
  + var.query.selectionOptions.withIncludeAll(false)
  + var.query.selectionOptions.withMulti(false)
  + var.query.refresh.onTime()
  + { allowCustomValue: false };

local descVar =
  var.query.new('description', 'openslo_slo_info{openslo_slo_name=~"$slo"}')
  + var.query.withDatasource(type='prometheus', uid='${datasource}')
  + var.query.withRegex('/.*openslo_slo_description="(.*)"/')
  + var.query.refresh.onTime()
  + var.query.selectionOptions.withIncludeAll(false)
  + var.query.selectionOptions.withMulti(false)
  + var.query.generalOptions.showOnDashboard.withNothing()
  + { allowCustomValue: false };

local targetVar =
  var.query.new('target', 'openslo_slo_objective{openslo_slo_name=~"$slo"}')
  + var.query.withDatasource(type='prometheus', uid='${datasource}')
  + var.query.withRegex('/.* (\\d+\\.?\\d*) .*/')
  + var.query.refresh.onTime()
  + var.query.selectionOptions.withIncludeAll(false)
  + var.query.selectionOptions.withMulti(false)
  + var.query.generalOptions.showOnDashboard.withNothing()
  + { allowCustomValue: false };

// ----- panels --------------------------------------------------------------

local textPanel =
  panel.text.new('')
  + panel.text.options.withMode('markdown')
  + panel.text.options.withContent('## $slo\n\n$description\n\n**Target:** $target')
  + panel.text.options.withCodeMixin({
    language: 'plaintext',
    showLineNumbers: false,
  })
  + panel.text.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_info{openslo_slo_name="$slo"}')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('A'),
    promQ.new('${datasource}', 'openslo_slo_objective{openslo_slo_name="$slo"}')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('B'),
  ])
  + panel.text.panelOptions.withGridPos(h=8, w=18, x=0, y=0)
  + { id: 1 };

local sloStat =
  panel.stat.new('SLO')
  + panel.stat.standardOptions.withUnit('percent')
  + panel.stat.standardOptions.withDecimals(2)
  + panel.stat.standardOptions.color.withMode('thresholds')
  + panel.stat.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.stat.options.withColorMode('value')
  + panel.stat.options.withGraphMode('area')
  + panel.stat.options.withJustifyMode('auto')
  + panel.stat.options.withOrientation('auto')
  + panel.stat.options.reduceOptions.withCalcs(['lastNotNull'])
  + panel.stat.options.reduceOptions.withFields('')
  + panel.stat.options.reduceOptions.withValues(false)
  + panel.stat.options.withTextMode('auto')
  + panel.stat.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_objective{openslo_slo_name="$slo"} * 100')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=6, x=18, y=0)
  + { datasource: ds, id: 10 };

local sliTs =
  panel.timeSeries.new('SLI')
  + panel.timeSeries.standardOptions.withUnit('percent')
  + panel.timeSeries.standardOptions.withDecimals(2)
  + panel.timeSeries.standardOptions.color.withMode('thresholds')
  + panel.timeSeries.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.timeSeries.fieldConfig.defaults.custom.withDrawStyle('line')
  + panel.timeSeries.fieldConfig.defaults.custom.withFillOpacity(10)
  + panel.timeSeries.fieldConfig.defaults.custom.withLineWidth(1)
  + panel.timeSeries.fieldConfig.defaults.custom.withShowPoints('never')
  + panel.timeSeries.options.legend.withDisplayMode('list')
  + panel.timeSeries.options.legend.withPlacement('bottom')
  + panel.timeSeries.options.legend.withShowLegend(true)
  + panel.timeSeries.options.legend.withCalcs([])
  + panel.timeSeries.options.tooltip.withMode('multi')
  + panel.timeSeries.options.tooltip.withSort('desc')
  + panel.timeSeries.queryOptions.withTargets([
    promQ.new('${datasource}', '(1 - openslo_sli_error_rate_30d{openslo_slo_name="$slo"}) * 100')
    + promQ.withLegendFormat('28d')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=18, x=0, y=8)
  + { datasource: ds, id: 11 };

local sli28Stat =
  panel.stat.new('28d SLI')
  + panel.timeSeries.standardOptions.withUnit('percent')
  + panel.stat.standardOptions.withDecimals(2)
  + panel.stat.standardOptions.color.withMode('thresholds')
  + panel.stat.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.stat.options.withColorMode('value')
  + panel.stat.options.withGraphMode('area')
  + panel.stat.options.withJustifyMode('auto')
  + panel.stat.options.withOrientation('auto')
  + panel.stat.options.reduceOptions.withCalcs(['lastNotNull'])
  + panel.stat.options.reduceOptions.withFields('')
  + panel.stat.options.reduceOptions.withValues(false)
  + panel.stat.options.withTextMode('auto')
  + panel.stat.queryOptions.withTargets([
    promQ.new('${datasource}', '(1 - openslo_sli_error_rate_30d{openslo_slo_name="$slo"}) * 100')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=6, x=18, y=16)
  + { datasource: ds, id: 12 };

local burndownTs =
  panel.timeSeries.new('Error Budget Burndown')
  + panel.timeSeries.standardOptions.withMin(0)
  + panel.timeSeries.standardOptions.withMax(100)
  + panel.timeSeries.standardOptions.withUnit('percent')
  + panel.timeSeries.standardOptions.withDecimals(2)
  + panel.timeSeries.standardOptions.color.withMode('thresholds')
  + panel.timeSeries.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.timeSeries.fieldConfig.defaults.custom.withDrawStyle('line')
  + panel.timeSeries.fieldConfig.defaults.custom.withFillOpacity(10)
  + panel.timeSeries.fieldConfig.defaults.custom.withLineWidth(1)
  + panel.timeSeries.fieldConfig.defaults.custom.withShowPoints('never')
  + panel.timeSeries.options.legend.withDisplayMode('list')
  + panel.timeSeries.options.legend.withPlacement('bottom')
  + panel.timeSeries.options.legend.withShowLegend(true)
  + panel.timeSeries.options.legend.withCalcs([])
  + panel.timeSeries.options.tooltip.withMode('multi')
  + panel.timeSeries.options.tooltip.withSort('desc')
  + panel.timeSeries.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_period_error_budget_remaining{openslo_slo_name="$slo"} * 100')
    + promQ.withLegendFormat('Period remaining')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=18, x=0, y=16)
  + { datasource: ds, id: 13 };

local remainingStat =
  panel.stat.new('28d Remaining Error Budget')
  + panel.stat.standardOptions.withUnit('percent')
  + panel.stat.standardOptions.withDecimals(2)
  + panel.stat.standardOptions.color.withMode('thresholds')
  + panel.stat.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.stat.options.withColorMode('value')
  + panel.stat.options.withGraphMode('area')
  + panel.stat.options.withJustifyMode('auto')
  + panel.stat.options.withOrientation('auto')
  + panel.stat.options.reduceOptions.withCalcs(['lastNotNull'])
  + panel.stat.options.reduceOptions.withFields('')
  + panel.stat.options.reduceOptions.withValues(false)
  + panel.stat.options.withTextMode('auto')
  + panel.stat.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_period_error_budget_remaining{openslo_slo_name="$slo"} * 100')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=6, x=18, y=24)
  + { datasource: ds, id: 14 };

local burnRateTs =
  panel.timeSeries.new('Error Budget Burn Rate')
  + panel.timeSeries.standardOptions.withDecimals(2)
  + panel.timeSeries.standardOptions.color.withMode('thresholds')
  + panel.timeSeries.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.timeSeries.fieldConfig.defaults.custom.withDrawStyle('line')
  + panel.timeSeries.fieldConfig.defaults.custom.withFillOpacity(10)
  + panel.timeSeries.fieldConfig.defaults.custom.withLineWidth(1)
  + panel.timeSeries.fieldConfig.defaults.custom.withShowPoints('never')
  + panel.timeSeries.options.legend.withDisplayMode('list')
  + panel.timeSeries.options.legend.withPlacement('bottom')
  + panel.timeSeries.options.legend.withShowLegend(true)
  + panel.timeSeries.options.legend.withCalcs([])
  + panel.timeSeries.options.tooltip.withMode('multi')
  + panel.timeSeries.options.tooltip.withSort('desc')
  + panel.timeSeries.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_current_burn_rate{openslo_slo_name="$slo"}')
    + promQ.withLegendFormat('Current burn rate')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=18, x=0, y=24)
  + { datasource: ds, id: 15 };

local currentBR =
  panel.stat.new('Current Burn Rate')
  + panel.stat.standardOptions.withDecimals(2)
  + panel.stat.standardOptions.color.withMode('thresholds')
  + panel.stat.standardOptions.thresholds.withSteps([{ color: 'green', value: null }])
  + panel.stat.options.withColorMode('value')
  + panel.stat.options.withGraphMode('area')
  + panel.stat.options.withJustifyMode('auto')
  + panel.stat.options.withOrientation('auto')
  + panel.stat.options.reduceOptions.withCalcs(['lastNotNull'])
  + panel.stat.options.reduceOptions.withFields('')
  + panel.stat.options.reduceOptions.withValues(false)
  + panel.stat.options.withTextMode('auto')
  + panel.stat.queryOptions.withTargets([
    promQ.new('${datasource}', 'openslo_slo_current_burn_rate{openslo_slo_name="$slo"}')
    + promQ.withLegendFormat('{{openslo_slo_name}}')
    + promQ.withRefId('A'),
  ])
  + panel.stat.panelOptions.withGridPos(h=8, w=6, x=18, y=32)
  + { datasource: ds, id: 16 };

// ----- dashboard -----------------------------------------------------------

g.dashboard.new('OpenSLO - SLO detail')
+ g.dashboard.withUid('openslo-detail')
+ g.dashboard.withDescription(
  'Per-SLO drilldown. Open from OpenSLO - Manage SLOs.'
)
+ g.dashboard.withTags(['openslo'])
+ g.dashboard.time.withFrom('now-30d')
+ g.dashboard.time.withTo('now')
+ g.dashboard.withRefresh('30s')
+ g.dashboard.withVariables([dsVar, sloVar, descVar, targetVar])
+ g.dashboard.withPanels([
  textPanel,
  sloStat,
  sliTs,
  sli28Stat,
  burndownTs,
  remainingStat,
  burnRateTs,
  currentBR,
])
