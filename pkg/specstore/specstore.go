package specstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenSLO/go-sdk/pkg/openslo"
	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/OpenSLO/go-sdk/pkg/openslosdk"
	"github.com/mdobak/go-xerrors"
	"github.com/thisisibrahimd/opensloctl/pkg/util"
)

type OpenSLOV1Specs struct {
	Services                 map[string]v1.Service
	SLOs                     map[string]v1.SLO
	SLIs                     map[string]v1.SLI
	DataSources              map[string]v1.DataSource
	AlertPolices             map[string]v1.AlertPolicy
	AlertConditions          map[string]v1.AlertCondition
	AlertNotificationTargets map[string]v1.AlertNotificationTarget
}

type OpenSLOSpecs struct {
	V1 OpenSLOV1Specs
}

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

var ERROR_SPEC_DUPLICATE = xerrors.New("")

func (s *OpenSLOSpecs) StoreSpec(o openslo.Object) error {
	if err := o.Validate(); err != nil {
		return xerrors.Newf("invalid spec %s/%s: %v", o.GetKind(), o.GetName(), err)
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
			s.V1.SLOs[o.GetName()] = o.(v1.SLO)
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

	// validate AlertCondition kind is burnrate
	for name, cond := range s.V1.AlertConditions {
		if cond.Spec.Condition.Kind != v1.AlertConditionKindBurnRate {
			errs = append(errs, xerrors.Newf("unsupported AlertCondition kind: AlertCondition %q has kind %q, only %q supported", name, cond.Spec.Condition.Kind, v1.AlertConditionKindBurnRate))
		}
	}

	if len(errs) > 0 {
		return xerrors.Join(errs)
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
