package slack

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"k8s.io/utils/strings/slices"
)

type messageFilterLayer struct {
	Conclusions []string `json:"conclusions"`
	Branches    []string `json:"branches"`
	Workflows   []string `json:"workflows"`
}

type MessageFilter struct {
	filters []messageFilterLayer
}

type exposedMessageFilter struct {
	Filters []messageFilterLayer `json:"filters"`
}

func (mf MessageFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(exposedMessageFilter{
		Filters: mf.filters,
	})
}

func (f *MessageFilter) UnmarshalJSON(data []byte) error {
	aux := &exposedMessageFilter{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	f.filters = aux.Filters
	return nil
}

func (mfl messageFilterLayer) Pass(run *jobs.WorkflowRun) bool {
	if len(mfl.Conclusions) > 0 && !slices.Contains(mfl.Conclusions, run.Conclusion) {
		return false
	}
	if len(mfl.Workflows) > 0 && !slices.Contains(mfl.Workflows, run.Name) {
		return false
	}
	return true
}

func (mf MessageFilter) Any(run *jobs.WorkflowRun) bool {
	if len(mf.filters) == 0 {
		return true
	}
	for _, mfl := range mf.filters {
		if mfl.Pass(run) {
			return true
		}
	}
	return false
}

func (mfl *messageFilterLayer) setConclusions(conclusions []string) error {
	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
	conclusionsEnum := []string{"action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out"}
	var unsupportedConclusions []string
	for _, c := range conclusions {
		if !slices.Contains(conclusionsEnum, c) {
			unsupportedConclusions = append(unsupportedConclusions, c)
		}
	}

	if len(unsupportedConclusions) > 0 {
		return fmt.Errorf("unsupported conclusions: %s", strings.Join(unsupportedConclusions, ", "))
	}

	mfl.Conclusions = conclusions
	return nil
}

func NewFilter(filterLayers []string) (*MessageFilter, error) {
	filter := MessageFilter{
		filters: []messageFilterLayer{},
	}

	for _, layer := range filterLayers {
		used := struct{ keys []string }{keys: []string{}}
		definition := strings.Split(layer, ":")

		switch len(definition) {
		case 1: // Assumed format "conclusion1,conclusion2,..."
			mfl := messageFilterLayer{}
			if slices.Contains(used.keys, "none") {
				return nil, fmt.Errorf("do not space-separate conclusions")
			}
			conclusions := strings.Split(definition[0], ",")

			err := mfl.setConclusions(conclusions)
			if err != nil {
				return nil, nil
			}

			used.keys = append(used.keys, "none")
			filter.filters = append(filter.filters, mfl)
		case 2, 3: // Assumed format "filterKey:filterValue1,filterValue2,..."
			mfl := messageFilterLayer{}
			filterType := definition[0]
			if slices.Contains(used.keys, filterType) {
				return nil, fmt.Errorf("duplicated filter type: %s", filterType)
			}
			switch filterType {
			case "branches":
				branches := strings.Split(definition[1], ",")
				mfl.Branches = branches
			case "workflows":
				workflows := strings.Split(definition[1], ",")
				mfl.Workflows = workflows
			default:
				return nil, fmt.Errorf("unsupported filter type: %s", filterType)
			}

			if len(definition) == 3 {
				conclusions := strings.Split(definition[2], ",")

				err := mfl.setConclusions(conclusions)
				if err != nil {
					return nil, nil
				}
			}

			used.keys = append(used.keys, filterType)
			filter.filters = append(filter.filters, mfl)
		}
	}

	return &filter, nil
}
