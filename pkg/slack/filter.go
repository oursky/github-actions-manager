package slack

import (
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"k8s.io/utils/strings/slices"
)

type MessageFilterRule struct {
	Conclusions []string `json:"conclusions"`
	// branches    []string
	Workflows []string `json:"workflows"`
}

type MessageFilter struct {
	Whitelists []MessageFilterRule `json:"whitelists"`
	// can be extended to include blacklists []messageFilterRule
}

func (rule MessageFilterRule) Pass(run *jobs.WorkflowRun) bool {
	if len(rule.Conclusions) > 0 && !slices.Contains(rule.Conclusions, run.Conclusion) {
		return false
	}
	if len(rule.Workflows) > 0 && !slices.Contains(rule.Workflows, run.Name) {
		return false
	}
	return true
}
func (mf MessageFilter) Any(run *jobs.WorkflowRun) bool {
	for _, rule := range mf.Whitelists {
		if rule.Pass(run) {
			return true
		}
	}
	return false
}

func (rule *MessageFilterRule) setConclusions(conclusions []string) error {
	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
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

	rule.Conclusions = conclusions
	return nil
}

func NewFilter(filterLayers []string) (*MessageFilter, error) {
	filter := MessageFilter{
		Whitelists: []MessageFilterRule{},
	}
	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
	for _, layer := range filterLayers {
		definition := strings.Split(layer, ":")

		definition = definition
		// switch definition[0]
		// case ""
	}

	return &filter, nil
}

// func (mf MessageFilter) String() string {
// 	output = ""
// }
