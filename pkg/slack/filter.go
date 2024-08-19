package slack

import (
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/samber/lo"
	"k8s.io/utils/strings/slices"
)

type MessageFilterRule struct {
	Conclusions []string `json:"conclusions"`
	Branches    []string `json:"branches"`
	Workflows   []string `json:"workflows"`
}

type MessageFilter struct {
	Whitelists []MessageFilterRule `json:"filters"`
	// can be extended to include blacklists []messageFilterRule
}

func (rule MessageFilterRule) String() string {
	output := ""
	if len(rule.Conclusions) > 0 {
		output += fmt.Sprintf("conclusions: %s", rule.Conclusions)
	}
	if len(rule.Branches) > 0 {
		output += fmt.Sprintf("branches: %s", rule.Branches)
	}
	if len(rule.Workflows) > 0 {
		output += fmt.Sprintf("workflows: %s", rule.Workflows)
	}
	return output
}

func (mf MessageFilter) String() string {
	return fmt.Sprintf("whitelists: %s", fmt.Sprintf("[%s]", strings.Join(lo.Map(mf.Whitelists, func(x MessageFilterRule, _ int) string { return x.String() }), ", ")))
}

func (mf MessageFilter) Length() int {
	return len(mf.Whitelists)
}

func (rule MessageFilterRule) Pass(run *jobs.WorkflowRun) bool {
	if len(rule.Conclusions) > 0 && !slices.Contains(rule.Conclusions, run.Conclusion) {
		return false
	}
	if len(rule.Branches) > 0 && !slices.Contains(rule.Branches, run.Branch) {
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

func (rule *MessageFilterRule) SetConclusions(conclusions []string) error {
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

	rule.Conclusions = conclusions
	return nil
}

func NewFilter(filterLayers []string) (*MessageFilter, error) {
	filter := MessageFilter{
		Whitelists: []MessageFilterRule{},
	}

	used := []string{}
	for _, layer := range filterLayers {
		definition := strings.Split(layer, ":")

		switch len(definition) {
		case 1: // Assumed format "conclusion1,conclusion2,..."
			rule := MessageFilterRule{}
			if slices.Contains(used, "none") {
				return nil, fmt.Errorf("duplicated conclusion strings; use commas to separate conclusions")
			}
			conclusions := strings.Split(definition[0], ",")

			err := rule.SetConclusions(conclusions)
			if err != nil {
				return nil, err
			}

			used = append(used, "none")
			filter.Whitelists = append(filter.Whitelists, rule)
		case 2, 3: // Assumed format "filterKey:filterValue1,filterValue2,..."
			rule := MessageFilterRule{}
			filterType := definition[0]
			if slices.Contains(used, filterType) {
				return nil, fmt.Errorf("duplicated filter type: %s", filterType)
			}
			switch filterType {
			case "branches":
				branches := strings.Split(definition[1], ",")
				rule.Branches = branches
			case "workflows":
				workflows := strings.Split(definition[1], ",")
				rule.Workflows = workflows
			default:
				return nil, fmt.Errorf("unsupported filter type: %s", filterType)
			}

			if len(definition) == 3 {
				conclusions := strings.Split(definition[2], ",")

				err := rule.SetConclusions(conclusions)
				if err != nil {
					return nil, err
				}
			}

			used = append(used, filterType)
			filter.Whitelists = append(filter.Whitelists, rule)
		}
	}

	return &filter, nil
}
