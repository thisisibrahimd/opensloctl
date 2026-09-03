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

	return specStore, nil
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
