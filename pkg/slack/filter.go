package slack

import (
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"k8s.io/utils/strings/slices"
)

type MessageFilterLayer struct {
	conclusions []string
	// branches    []string
	workflows []string
}
type MessageFilter struct {
	filters []MessageFilterLayer
}

func (mfl MessageFilterLayer) Pass(run *jobs.WorkflowRun) bool {
	if len(mfl.conclusions) > 0 && !slices.Contains(mfl.conclusions, run.Conclusion) {
		return false
	}
	if len(mfl.workflows) > 0 && !slices.Contains(mfl.workflows, run.Name) {
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

func (mfl *MessageFilterLayer) setConclusions(conclusions []string) error {
	conclusionsEnum := []string{"action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out"}
	// var supportedConclusions, unsupportedConclusions []string
	var unsupportedConclusions []string
	for _, c := range conclusions {
		// if slices.Contains(conclusionsEnum, c) {
		// 	supportedConclusions = append(supportedConclusions, c)
		// } else {
		if !slices.Contains(conclusionsEnum, c) {
			unsupportedConclusions = append(unsupportedConclusions, c)
		}
	}

	if len(unsupportedConclusions) > 0 {
		if slices.Contains(unsupportedConclusions, " ") {
			return fmt.Errorf("Do not space-separate conclusions. Use format conclusion1,conclusion2")
		}
		return fmt.Errorf("unsupported conclusions: %s", strings.Join(unsupportedConclusions, ", "))
	}

	mfl.conclusions = conclusions
	return nil
}

func NewFilter(filterLayers []string) (MessageFilter, error) {
	filter := MessageFilter{
		filters: []MessageFilterLayer{},
	}
	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
	for _, layer := range filterLayers {
		definition := strings.Split(layer, ":")

		definition = definition
		// switch definition[0]
		// case ""
	}

	return filter, nil
}

// func (mf MessageFilter) String() string {
// 	output = ""
// }
