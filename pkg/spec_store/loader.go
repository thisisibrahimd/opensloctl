package spec_store

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
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

func (s *SpecStore) readSpecs() ([]openslo.Object, error) {
	// remove loaded specs
	s.clearStore()

	// read files and dirs
	log.Info("finding files")
	filenames, err := util.FindFiles(s.filenames, s.recursive)
	if err != nil {
		log.Error("unable to read files in filenames provided", "err", err)
	}

	// read files
	log.Info("reading and decoding files")
	var opensloObjects []openslo.Object
	for _, filename := range filenames {
		log.Info("reading file", "file", filename)
		file, err := os.ReadFile(filepath.Clean(filename))
		if err != nil {
			log.Error("unable to read file", "file", file)
			continue
		}

		decoder := bytes.NewBuffer(file)
		objects, err := openslosdk.Decode(decoder, openslosdk.FormatYAML)
		if err != nil {
			log.Error("unable to decode file to openslo spec. skipping", "file", filename)
			continue
		}

		log.Debug("successfully loaded file", "file", filename)
		opensloObjects = append(opensloObjects, objects...)
	}

	log.Info("finished reading and decoding files into openslo specs", "amount", len(opensloObjects))
	return opensloObjects, nil
}

func (s *SpecStore) clearStore() {
	s.Store.V1.Services = []v1.Service{}
	s.Store.V1.SLOs = []v1.SLO{}
	s.Store.V1.SLIs = []v1.SLI{}
	s.Store.V1.DataSources = []v1.DataSource{}
	s.Store.V1.AlertPolicy = []v1.AlertPolicy{}
	s.Store.V1.AlertConditions = []v1.AlertCondition{}
	s.Store.V1.AlertNotificationTargets = []v1.AlertNotificationTarget{}
	s.Store.V1.BudgetAdjustments = []v1.BudgetAdjustment{}

}

func (s *SpecStore) LoadSpecs() {
	log.Info("organizing and storing specs")

	opensloObjects, err := s.readSpecs()
	if err != nil {
		log.Fatal("unable to read specs", "err", err)
	}

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
