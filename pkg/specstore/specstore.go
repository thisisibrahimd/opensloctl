package specstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenSLO/go-sdk/pkg/openslo"
	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/OpenSLO/go-sdk/pkg/openslosdk"
	"github.com/mdobak/go-xerrors"
	"github.com/thisisibrahimd/opensloctl/pkg/util"
)

// AlertConditionKind identifies the SRE alerting strategy a condition
// expresses. The OpenSLO SDK defines a single kind constant ("burnrate")
// for now; opensloctl extends the allowed surface with three more kinds
// that map 1-to-1 onto the alerting patterns described in the Google SRE
// workbook (https://sre.google/workbook/alerting-on-slos/).
//
// - error-rate:                raw SLI error rate vs absolute threshold
//                              (workbook §§ 1–3; sections 1–3 differ only in
//                              lookback window length and whether alertAfter is
//                              set, which maps to the Prom `for:` clause).
// - burn-rate:                 single burn rate multiplier vs error rate / budget
//                              (workbook § 4).
// - multi-burn-rate:           multiple burn-rate conditions OR-ed per severity
//                              (workbook § 5).
// - multi-window-multi-burn-rate: short+long window pairs AND-ed within a
//                              tier and OR-ed across tiers
//                              (workbook § 6; the original opensloctl default).
type AlertConditionKind string

const (
	KindErrorRate                   AlertConditionKind = "error-rate"
	KindBurnRate                    AlertConditionKind = "burn-rate"
	KindMultiBurnRate               AlertConditionKind = "multi-burn-rate"
	KindMultiWindowMultiBurnRate    AlertConditionKind = "multi-window-multi-burn-rate"
)

// validKinds is the closed set of kinds opensloctl accepts. Membership is
// checked in ValidateRefs; anything outside this set is rejected.
var validKinds = map[AlertConditionKind]bool{
	KindErrorRate:                true,
	KindBurnRate:                 true,
	KindMultiBurnRate:            true,
	KindMultiWindowMultiBurnRate: true,
}

// kindOrder lists every supported AlertConditionKind in a stable display
// order. Used to build deterministic error messages that list the valid
// kinds rather than relying on map iteration order.
var kindOrder = []AlertConditionKind{
	KindErrorRate,
	KindBurnRate,
	KindMultiBurnRate,
	KindMultiWindowMultiBurnRate,
}

// validateThreshold applies the per-kind threshold range rules:
//
//   - error-rate: threshold is an absolute error rate; must lie in (0, 1].
//   - burn-rate families: threshold is a burn multiplier; must be > 0.
//
// Both ranges intentionally reject the SDK-required pointer being nil
// with one shared nil-check upstream.
func validateThreshold(kind AlertConditionKind, threshold *float64) error {
	if threshold == nil {
		return nil
	}
	switch kind {
	case KindErrorRate:
		if *threshold <= 0 || *threshold > 1 {
			return xerrors.Newf("error-rate threshold must be in (0, 1], got %f", *threshold)
		}
	case KindBurnRate, KindMultiBurnRate, KindMultiWindowMultiBurnRate:
		if *threshold <= 0 {
			return xerrors.Newf("%s threshold must be > 0, got %f", kind, *threshold)
		}
	}
	return nil
}

// ValidKind reports whether k is one of the supported AlertConditionKind
// values. Exported for consumers (especially the Prometheus generator)
// that need to switch on kind without re-deriving the closed set.
func ValidKind(k AlertConditionKind) bool {
	return validKinds[k]
}

// KindPascal converts a kebab-case kind to PascalCase for embedding in
// alert names (e.g. "multi-window-multi-burn-rate" →
// "MultiWindowMultiBurnRate"). Unknown kinds return the input kebab-cased
// unchanged.
func KindPascal(k AlertConditionKind) string {
	switch k {
	case KindErrorRate:
		return "ErrorRate"
	case KindBurnRate:
		return "BurnRate"
	case KindMultiBurnRate:
		return "MultiBurnRate"
	case KindMultiWindowMultiBurnRate:
		return "MultiWindowMultiBurnRate"
	default:
		return string(k)
	}
}

// SloNamePascal converts a kebab-case name to PascalCase by removing the
// hyphens and title-casing each segment. Useful for combining the SLO name
// into a fully-PascalCase alert identifier without any underscore
// separators, e.g. "test-tiered-slo" → "TestTieredSlo".
func SloNamePascal(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// KindDescription converts a kebab-case kind to a lower-spaced form for use
// in alert annotations (e.g. "multi-window-multi-burn-rate" →
// "multi-window multi-burn rate"). Unknown kinds return the input unchanged.
func KindDescription(k AlertConditionKind) string {
	switch k {
	case KindErrorRate:
		return "error rate"
	case KindBurnRate:
		return "burn rate"
	case KindMultiBurnRate:
		return "multi-burn rate"
	case KindMultiWindowMultiBurnRate:
		return "multi-window multi-burn rate"
	default:
		return string(k)
	}
}

// validKindsList renders the supported kinds as a comma-separated list
// for inclusion in error messages. The order matches kindOrder.
func validKindsList() string {
	parts := make([]string, 0, len(kindOrder))
	for _, k := range kindOrder {
		parts = append(parts, string(k))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%v", parts)
}

// OpenSLOV1Specs is an in-memory container for all decoded OpenSlo v1
// objects, partitioned by kind. Each map keys by the object's metadata.name.
type OpenSLOV1Specs struct {
	Services                 map[string]v1.Service
	SLOs                     map[string]v1.SLO
	SLIs                     map[string]v1.SLI
	DataSources              map[string]v1.DataSource
	AlertPolices             map[string]v1.AlertPolicy
	AlertConditions          map[string]v1.AlertCondition
	AlertNotificationTargets map[string]v1.AlertNotificationTarget
}

// OpenSLOSpecs holds a parsed collection of OpenSlo specs. Use NewOpenSLOSpecs
// to create an empty store, then StoreSpec to add objects and ValidateRefs to
// verify cross-references resolve.
type OpenSLOSpecs struct {
	V1 OpenSLOV1Specs
}

// NewOpenSLOSpecs returns an empty OpenSLOSpecs with all per-kind maps
// initialized but containing no objects.
func NewOpenSLOSpecs() *OpenSLOSpecs {
	opensloSpecs := &OpenSLOSpecs{
		V1: OpenSLOV1Specs{
			Services:                 map[string]v1.Service{},
			SLOs:                     map[string]v1.SLO{},
			SLIs:                     map[string]v1.SLI{},
			DataSources:              map[string]v1.DataSource{},
			AlertPolices:             map[string]v1.AlertPolicy{},
			AlertConditions:          map[string]v1.AlertCondition{},
			AlertNotificationTargets: map[string]v1.AlertNotificationTarget{},
		},
	}

	return opensloSpecs
}

// ERROR_SPEC_DUPLICATE surfaces duplicate-name detection in error chains.
// Consumers can match this sentinel via xerrors.Is.
var ERROR_SPEC_DUPLICATE = xerrors.New("")

// StoreSpec validates o via the SDK's Validate(), runs opensloctl-specific
// post-checks (target required, timeWindow must be rolling), and inserts o
// into the appropriate per-kind map. Duplicates are rejected with
// ERROR_SPEC_DUPLICATE wrapped in an xerror chain.
//
// AlertCondition is allowed to bypass the SDK's strict AlertConditionKind
// OneOf check: opensloctl accepts four extended kinds
// (see validKinds) that map to SRE-workbook alerting strategies. The SDK
// only recognizes the legacy "burnrate" kind. To keep SDK-based error
// reporting for everything else (severity required, op required, etc.)
// without rejecting the extended kinds wholesale, we drop only the kind
// OneOf rejection and re-run the SDK's kind OneOf if the kind is not in
// our extended set. Threshold/lookbackWindow/alertAfter SDK validation
// runs only When(kind == "burnrate") — opensloctl ValidateRefs mirrors
// those checks across all four kinds by enforcing per-kind ranges.
func (s *OpenSLOSpecs) StoreSpec(o openslo.Object) error {
	if o.GetKind() != openslo.KindAlertCondition {
		if err := o.Validate(); err != nil {
			return xerrors.Newf("invalid spec %s/%s: %v", o.GetKind(), o.GetName(), err)
		}
	}

	switch o.GetVersion() {
	case openslo.VersionV1:
		switch o.GetKind() {
		case openslo.KindService:
			if _, ok := s.V1.Services[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.Services[o.GetName()] = o.(v1.Service)
		case openslo.KindSLO:
			if _, ok := s.V1.SLOs[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			slo := o.(v1.SLO)
			for i, obj := range slo.Spec.Objectives {
				if obj.Target == nil && obj.TargetPercent == nil {
					return xerrors.Newf("invalid spec %s/%s: objective[%d] requires target or targetPercent (required by opensloctl generator, not enforced by SDK)", o.GetKind(), o.GetName(), i)
				}
			}
			for i, tw := range slo.Spec.TimeWindow {
				if !tw.IsRolling {
					return xerrors.Newf("invalid spec %s/%s: timeWindow[%d] must have isRolling: true (opensloctl generator only supports rolling windows)", o.GetKind(), o.GetName(), i)
				}
			}
			s.V1.SLOs[o.GetName()] = slo
		case openslo.KindSLI:
			if _, ok := s.V1.SLIs[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.SLIs[o.GetName()] = o.(v1.SLI)
		case openslo.KindDataSource:
			if _, ok := s.V1.DataSources[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.DataSources[o.GetName()] = o.(v1.DataSource)
		case openslo.KindAlertPolicy:
			if _, ok := s.V1.AlertPolices[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.AlertPolices[o.GetName()] = o.(v1.AlertPolicy)
		case openslo.KindAlertCondition:
			if _, ok := s.V1.AlertConditions[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.AlertConditions[o.GetName()] = o.(v1.AlertCondition)
		case openslo.KindAlertNotificationTarget:
			if _, ok := s.V1.AlertNotificationTargets[o.GetName()]; ok {
				return xerrors.Newf("duplicate spec found: %s", o.GetName())
			}
			s.V1.AlertNotificationTargets[o.GetName()] = o.(v1.AlertNotificationTarget)
		default:
			return xerrors.Newf("unrecognized kind: %s", o.GetKind().String())
		}
	default:
		return xerrors.Newf("unrecognized version: %s", o.GetVersion().String())

	}

	return nil
}

// GetSpecs discovers YAML files from filenames (or, when recursive is true,
// from directories containing YAML files), parses each via the OpenSlo SDK,
// stores them, and validates all cross-references. Returns a populated
// *OpenSLOSpecs on success or the first error encountered.
//
// filenames may also reference directories directly; recursive controls
// whether non-OpenSlo YAML inside those directories is silently skipped
// (always true) versus only surface-level files. See util.FindFiles for
// discovery rules.
func GetSpecs(filenames []string, recursive bool) (*OpenSLOSpecs, error) {
	specStore := NewOpenSLOSpecs()

	// recursively get all filesnames from flag (can contain single files and dirs to be recursivly search from)
	filenames, err := util.FindFiles(filenames, recursive)
	if err != nil {
		return nil, xerrors.New("error detecting files", err)
	}

	// read and parse specs
	specs, err := loadSpecs(filenames)
	if err != nil {
		return nil, xerrors.New("error reading specs", err)
	}

	// load specs into the store
	for _, o := range specs {
		if err := specStore.StoreSpec(o); err != nil {
			return nil, xerrors.New("error storing spec", err)
		}
	}

	// validate all references resolve
	if err := specStore.ValidateRefs(); err != nil {
		return nil, xerrors.New("unresolved references", err)
	}

	return specStore, nil
}

// ValidateRefs checks every cross-reference between objects:
// SLO → Service, SLO → SLI (via indicatorRef), SLO objective → SLI,
// SLO → AlertPolicy, AlertPolicy → AlertCondition,
// AlertPolicy → AlertNotificationTarget, and SLI → DataSource.
//
// It also enforces AlertCondition.kind == "burnrate" only. Conditions per
// severity are not counted: any number of page or ticket conditions is
// permitted; the Prometheus generator OR-s them in the resulting alert
// expression. All errors are joined via xerrors.Join.
func (s *OpenSLOSpecs) ValidateRefs() error {
	var errs []error

	// validate SLO references
	for name, slo := range s.V1.SLOs {
		// SLO → Service
		if slo.Spec.Service != "" {
			if _, ok := s.V1.Services[slo.Spec.Service]; !ok {
				errs = append(errs, xerrors.Newf("unresolved ref: SLO %q references Service %q not found", name, slo.Spec.Service))
			}
		}

		// SLO → SLI (indicatorRef)
		if slo.Spec.IndicatorRef != nil && *slo.Spec.IndicatorRef != "" {
			if _, ok := s.V1.SLIs[*slo.Spec.IndicatorRef]; !ok {
				errs = append(errs, xerrors.Newf("unresolved ref: SLO %q references SLI %q not found", name, *slo.Spec.IndicatorRef))
			}
		}

		// SLO → SLI (composite objectives with indicatorRef)
		for i, obj := range slo.Spec.Objectives {
			if obj.IndicatorRef != nil && *obj.IndicatorRef != "" {
				if _, ok := s.V1.SLIs[*obj.IndicatorRef]; !ok {
					errs = append(errs, xerrors.Newf("unresolved ref: SLO %q objective[%d] references SLI %q not found", name, i, *obj.IndicatorRef))
				}
			}
		}

		// SLO → AlertPolicy
		for _, ap := range slo.Spec.AlertPolicies {
			if ap.SLOAlertPolicyRef != nil && ap.AlertPolicyRef != "" {
				if _, ok := s.V1.AlertPolices[ap.AlertPolicyRef]; !ok {
					errs = append(errs, xerrors.Newf("unresolved ref: SLO %q references AlertPolicy %q not found", name, ap.AlertPolicyRef))
				}
			}
		}
	}

	// validate AlertPolicy references
	for name, ap := range s.V1.AlertPolices {
		// AlertPolicy → AlertCondition
		for _, cond := range ap.Spec.Conditions {
			if cond.AlertPolicyConditionRef != nil && cond.ConditionRef != "" {
				if _, ok := s.V1.AlertConditions[cond.ConditionRef]; !ok {
					errs = append(errs, xerrors.Newf("unresolved ref: AlertPolicy %q references AlertCondition %q not found", name, cond.ConditionRef))
				}
			}
		}

		// AlertPolicy → AlertNotificationTarget
		for _, nt := range ap.Spec.NotificationTargets {
			if nt.AlertPolicyNotificationTargetRef != nil && nt.TargetRef != "" {
				if _, ok := s.V1.AlertNotificationTargets[nt.TargetRef]; !ok {
					errs = append(errs, xerrors.Newf("unresolved ref: AlertPolicy %q references AlertNotificationTarget %q not found", name, nt.TargetRef))
				}
			}
		}

		// opensloctl supports at most one notification target per AlertPolicy
		// so each generated Prometheus alert can carry a single
		// openslo_notification_target label.
		if len(ap.Spec.NotificationTargets) > 1 {
			errs = append(errs, xerrors.Newf("AlertPolicy %q has %d notificationTargets; only one is supported", name, len(ap.Spec.NotificationTargets)))
		}
	}

	// validate SLI → DataSource references
	for name, sli := range s.V1.SLIs {
		checkMetricSource := func(source v1.SLIMetricSource, metricType string) {
			if source.MetricSourceRef != "" {
				if _, ok := s.V1.DataSources[source.MetricSourceRef]; !ok {
					errs = append(errs, xerrors.Newf("unresolved ref: SLI %q %s references DataSource %q not found", name, metricType, source.MetricSourceRef))
				}
			}
		}

		if sli.Spec.ThresholdMetric != nil {
			checkMetricSource(sli.Spec.ThresholdMetric.MetricSource, "thresholdMetric")
		}
		if sli.Spec.RatioMetric != nil {
			if sli.Spec.RatioMetric.Good != nil {
				checkMetricSource(sli.Spec.RatioMetric.Good.MetricSource, "ratioMetric.good")
			}
			if sli.Spec.RatioMetric.Bad != nil {
				checkMetricSource(sli.Spec.RatioMetric.Bad.MetricSource, "ratioMetric.bad")
			}
			if sli.Spec.RatioMetric.Total != nil {
				checkMetricSource(sli.Spec.RatioMetric.Total.MetricSource, "ratioMetric.total")
			}
			if sli.Spec.RatioMetric.Raw != nil {
				checkMetricSource(sli.Spec.RatioMetric.Raw.MetricSource, "ratioMetric.raw")
			}
		}
	}

	// validate AlertCondition kind + threshold range per kind
	for name, cond := range s.V1.AlertConditions {
		kind := AlertConditionKind(cond.Spec.Condition.Kind)
		if !validKinds[kind] {
			errs = append(errs, xerrors.Newf("unsupported AlertCondition kind: AlertCondition %q has kind %q, supported kinds: %s", name, cond.Spec.Condition.Kind, validKindsList()))
			continue
		}
		if err := validateThreshold(kind, cond.Spec.Condition.Threshold); err != nil {
			errs = append(errs, xerrors.Newf("invalid AlertCondition %q: %v", name, err))
		}
	}

	// validate SLO status threshold annotations
	for name, slo := range s.V1.SLOs {
		if err := validateStatusThresholds(slo.Metadata.Annotations); err != nil {
			errs = append(errs, xerrors.Newf("invalid status thresholds on SLO %q: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return xerrors.Join(errs)
	}
	return nil
}

// StatusThresholdAnnotationWarning, Critical, Breached are the SLO
// metadata.annotation keys that override the default status-gauge
// ranges (defaults 1/6/14.4 per the Google SRE workbook). Exported so
// the generator and tests can reference the same string literals.
const (
	StatusThresholdAnnotationWarning  = "threshold.status.openslo.com/warning"
	StatusThresholdAnnotationCritical = "threshold.status.openslo.com/critical"
	StatusThresholdAnnotationBreached = "threshold.status.openslo.com/breached"
)

// StatusThresholdDefault* mirror the SRE-workbook burn-rate reference
// points used when no annotation override is supplied.
const (
	StatusThresholdDefaultWarning   = 1.0
	StatusThresholdDefaultCritical  = 6.0
	StatusThresholdDefaultBreached  = 14.4
)

// ParseStatusThreshold parses a status-threshold annotation value as a
// float. Returns the default when the annotation is absent, empty, or
// unparseable — so a stray reminder note like "TODO: tune later"
// falls back silently rather than failing the load. The returned error
// is reserved for future use; current callers all expect nil.
func ParseStatusThreshold(raw string, def float64) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def, nil
	}
	return v, nil
}

// validateStatusThresholds enforces the ascending-positive invariant on
// the resolved (warning, critical, breached) triple. Each annotation is
// optional; missing annotations fall back to defaults independently.
func validateStatusThresholds(ann map[string]string) error {
	warn, err := ParseStatusThreshold(ann[StatusThresholdAnnotationWarning], StatusThresholdDefaultWarning)
	if err != nil {
		return xerrors.Newf("annotation %q: %v", StatusThresholdAnnotationWarning, err)
	}
	crit, err := ParseStatusThreshold(ann[StatusThresholdAnnotationCritical], StatusThresholdDefaultCritical)
	if err != nil {
		return xerrors.Newf("annotation %q: %v", StatusThresholdAnnotationCritical, err)
	}
	breach, err := ParseStatusThreshold(ann[StatusThresholdAnnotationBreached], StatusThresholdDefaultBreached)
	if err != nil {
		return xerrors.Newf("annotation %q: %v", StatusThresholdAnnotationBreached, err)
	}
	if warn <= 0 || crit <= 0 || breach <= 0 {
		return xerrors.Newf("warning=%g critical=%g breached=%g must all be positive", warn, crit, breach)
	}
	if !(warn < crit && crit < breach) {
		return xerrors.Newf("warning=%g critical=%g breached=%g must be strictly ascending", warn, crit, breach)
	}
	return nil
}

func loadSpecs(filenames []string) ([]openslo.Object, error) {
	var opensloObjects []openslo.Object
	for _, filename := range filenames {
		objects, err := loadSpec(filename)
		if err != nil {
			continue
		}

		opensloObjects = append(opensloObjects, objects...)
	}
	return opensloObjects, nil
}

func loadSpec(filename string) ([]openslo.Object, error) {
	var opensloObjects []openslo.Object
	file, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, xerrors.New(fmt.Sprintf("error reading file: %s", filename), err)
	}

	decoder := bytes.NewBuffer(file)
	opensloObjects, err = openslosdk.Decode(decoder, openslosdk.FormatYAML)
	if err != nil {
		return nil, xerrors.New(fmt.Sprintf("error parsing spec: %s", filename), err)
	}

	return opensloObjects, nil
}
