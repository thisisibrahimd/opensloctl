package spec_store

import (
	"bytes"
	"log"

	"github.com/thisisibrahimd/openslo/pkg/openslo"
	v1 "github.com/thisisibrahimd/openslo/pkg/openslo/v1"
	"github.com/thisisibrahimd/openslo/pkg/openslosdk"
	"github.com/thisisibrahimd/opensloctl/pkg/util"
)

type SpecStore struct {
	filenames []string
	recursive bool
	Store     struct {
		V1 struct {
			Services                 []v1.Service
			SLOs                     []v1.SLO
			SLIs                     []v1.SLI
			DataSources              []v1.DataSource
			AlertPolicy              []v1.AlertPolicy
			AlertConditions          []v1.AlertCondition
			AlertNotificationTargets []v1.AlertNotificationTarget
			BudgetAdjustments        []v1.BudgetAdjustment
		}
	}
}

func NewSpecStore(filenames []string, recursive bool) *SpecStore {
	return &SpecStore{
		filenames: filenames,
		recursive: recursive,
	}
}

func (s *SpecStore) getSpecs() ([]openslo.Object, error) {
	files, err := util.LoadFiles(s.filenames, s.recursive)
	if err != nil {
		return nil, err
	}

	// decode files
	var opensloObjects []openslo.Object
	for _, file := range files {
		decoder := bytes.NewBuffer(file)
		objects, err := openslosdk.Decode(decoder, openslosdk.FormatYAML)
		if err != nil {
			return nil, err
		}
		opensloObjects = append(opensloObjects, objects...)
	}

	return opensloObjects, nil
}

func (s *SpecStore) LoadSpecs() {
	opensloObjects, err := s.getSpecs()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("organizing and storing specs")

	for _, spec := range opensloObjects {
		switch version := spec.GetVersion(); version {
		case openslo.VersionV1:
			switch kind := spec.GetKind(); kind {
			case openslo.KindService:
				s.Store.V1.Services = append(s.Store.V1.Services, spec.(v1.Service))
			case openslo.KindSLO:
				s.Store.V1.SLOs = append(s.Store.V1.SLOs, spec.(v1.SLO))
			case openslo.KindSLI:
				s.Store.V1.SLIs = append(s.Store.V1.SLIs, spec.(v1.SLI))
			case openslo.KindDataSource:
				s.Store.V1.DataSources = append(s.Store.V1.DataSources, spec.(v1.DataSource))
			case openslo.KindAlertPolicy:
				s.Store.V1.AlertPolicy = append(s.Store.V1.AlertPolicy, spec.(v1.AlertPolicy))
			case openslo.KindAlertCondition:
				s.Store.V1.AlertConditions = append(s.Store.V1.AlertConditions, spec.(v1.AlertCondition))
			case openslo.KindAlertNotificationTarget:
				s.Store.V1.AlertNotificationTargets = append(s.Store.V1.AlertNotificationTargets, spec.(v1.AlertNotificationTarget))
			case openslo.KindBudgetAdjustment:
				s.Store.V1.BudgetAdjustments = append(s.Store.V1.BudgetAdjustments, spec.(v1.BudgetAdjustment))
			}
		}
	}
}
