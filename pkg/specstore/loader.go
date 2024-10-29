package specstore

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

type OpenSloSpecs struct {
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

type SpecStore struct {
	filenames []string
	recursive bool
}

type SpecStoreOption func(*SpecStore)

func WithFilenames(filenames []string) SpecStoreOption {
	return func(ss *SpecStore) {
		ss.filenames = filenames
	}
}

func WithRecursive(recursive bool) SpecStoreOption {
	return func(ss *SpecStore) {
		ss.recursive = recursive
	}
}

func NewSpecStore(options ...SpecStoreOption) *SpecStore {
	specStore := &SpecStore{
		filenames: []string{},
		recursive: false,
	}

	for _, opt := range options {
		opt(specStore)
	}

	return specStore
}

func (s *SpecStore) GetSpecs() (*OpenSloSpecs, error) {
	openSloObjects, err := s.loadSpecs()
	if err != nil {
		return nil, err
	}

	sortedOpenSloSpecs := s.sortSpecs(openSloObjects)

	return sortedOpenSloSpecs, nil
}

func (s *SpecStore) loadSpecs() ([]openslo.Object, error) {
	log.Info("loading specs")
	filenames, err := util.FindFiles(s.filenames, s.recursive)
	if err != nil {
		log.Error("unable to read files in filenames provided", "err", err)
		return nil, err
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

func (s *SpecStore) sortSpecs(openSloObjects []openslo.Object) *OpenSloSpecs {
	log.Info("sorting specs")
	specs := &OpenSloSpecs{}

	for _, spec := range openSloObjects {
		switch version := spec.GetVersion(); version {
		case openslo.VersionV1:
			switch kind := spec.GetKind(); kind {
			case openslo.KindService:
				specs.V1.Services = append(specs.V1.Services, spec.(v1.Service))
			case openslo.KindSLO:
				specs.V1.SLOs = append(specs.V1.SLOs, spec.(v1.SLO))
			case openslo.KindSLI:
				specs.V1.SLIs = append(specs.V1.SLIs, spec.(v1.SLI))
			case openslo.KindDataSource:
				specs.V1.DataSources = append(specs.V1.DataSources, spec.(v1.DataSource))
			case openslo.KindAlertPolicy:
				specs.V1.AlertPolicy = append(specs.V1.AlertPolicy, spec.(v1.AlertPolicy))
			case openslo.KindAlertCondition:
				specs.V1.AlertConditions = append(specs.V1.AlertConditions, spec.(v1.AlertCondition))
			case openslo.KindAlertNotificationTarget:
				specs.V1.AlertNotificationTargets = append(specs.V1.AlertNotificationTargets, spec.(v1.AlertNotificationTarget))
			case openslo.KindBudgetAdjustment:
				specs.V1.BudgetAdjustments = append(specs.V1.BudgetAdjustments, spec.(v1.BudgetAdjustment))
			}
		}
	}

	return specs
}
