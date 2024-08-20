package slack

import (
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/samber/lo"
	"k8s.io/utils/strings/slices"
)

type Conclusion string

const (
	ConclusionActionRequired Conclusion = "action_required"
	ConclusionCancelled      Conclusion = "cancelled"
	ConclusionFailure        Conclusion = "failure"
	ConclusionNeutral        Conclusion = "neutral"
	ConclusionSuccess        Conclusion = "success"
	ConclusionSkipped        Conclusion = "skipped"
	ConclusionStale          Conclusion = "stale"
	ConclusionTimedOut       Conclusion = "timed_out"
)

type MessageFilterRule struct {
	Conclusions []Conclusion `json:"conclusions"`
	Branches    []string     `json:"branches"`
	Workflows   []string     `json:"workflows"`
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

func (rule MessageFilterRule) Pass(run *jobs.WorkflowRun, conclusion Conclusion) bool {
	if len(rule.Conclusions) > 0 && !lo.Contains(rule.Conclusions, conclusion) {
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

func (mf MessageFilter) Any(run *jobs.WorkflowRun) (bool, error) {
	conclusion, err := NewConclusionFromString(run.Conclusion)
	if err != nil {
		return false, fmt.Errorf("Workflow run yielded invalid conclusion: %s", conclusion)
	}
	for _, rule := range mf.Whitelists {
		if rule.Pass(run, conclusion) {
			return true, nil
		}
	}
	return false, nil
}

func NewConclusionFromString(str string) (Conclusion, error) {
	switch str {
	case "action_required":
		return ConclusionActionRequired, nil
	case "cancelled":
		return ConclusionCancelled, nil
	case "failure":
		return ConclusionFailure, nil
	case "neutral":
		return ConclusionNeutral, nil
	case "success":
		return ConclusionSuccess, nil
	case "skipped":
		return ConclusionSkipped, nil
	case "stale":
		return ConclusionStale, nil
	case "timed_out":
		return ConclusionTimedOut, nil
	default:
		return "", fmt.Errorf("unknown conclusion: %s", str)
	}
}

func ParseConclusions(conclusionStrings []string) ([]Conclusion, error) {
	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
	// conclusionsEnum := []string{"action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out"}
	var conclusions []Conclusion
	var unsupportedConclusions []string
	for _, c := range conclusionStrings {
		conclusion, err := NewConclusionFromString(c)
		if err != nil {
			return nil, err
		}
		conclusions = append(conclusions, conclusion)
	}

	if len(unsupportedConclusions) > 0 {
		return nil, fmt.Errorf("unsupported conclusions: %s", strings.Join(unsupportedConclusions, ", "))
	}

	return conclusions, nil
}

func NewFilterRule(key string, values []string, conclusions []Conclusion) (*MessageFilterRule, error) {
	mfr := &MessageFilterRule{}
	switch key {
	case "conclusions":
	case "workflows":
		mfr.Workflows = values
	case "branches":
		mfr.Branches = values
	default:
		return nil, fmt.Errorf("unsupported filter type: %s", key)
	}

	mfr.Conclusions = conclusions
	return mfr, nil
}

func NewFilter(whitelists []MessageFilterRule) MessageFilter {
	return MessageFilter{
		Whitelists: whitelists,
	}
}

func ParseAsFilter(filterRuleStrings []string) (*MessageFilter, error) {
	whitelists := []MessageFilterRule{}
	used := []string{}
	for _, ruleString := range filterRuleStrings {
		definition := strings.Split(ruleString, ":")

		switch len(definition) {
		case 1: // Assumed format "conclusion1,conclusion2,..."
			if slices.Contains(used, "none") {
				return nil, fmt.Errorf("duplicated conclusion strings; use commas to separate conclusions")
			}

			conclusionStrings := strings.Split(definition[0], ",")

			conclusions, err := ParseConclusions(conclusionStrings)
			if err != nil {
				return nil, err
			}

			rule, err := NewFilterRule("conclusions", []string{}, conclusions)
			if err != nil {
				return nil, err
			}

			used = append(used, "none")
			whitelists = append(whitelists, *rule)
		case 2: // Assumed format "filterKey:filterValue1,filterValue2,..."
			filterType := definition[0]
			if slices.Contains(used, filterType) {
				return nil, fmt.Errorf("duplicated filter type: %s", filterType)
			}

			values := strings.Split(definition[1], ",")
			rule, err := NewFilterRule(filterType, values, []Conclusion{})
			if err != nil {
				return nil, err
			}

			used = append(used, filterType)
			whitelists = append(whitelists, *rule)
		case 3:
			filterType := definition[0]
			if slices.Contains(used, filterType) {
				return nil, fmt.Errorf("duplicated filter type: %s", filterType)
			}

			values := strings.Split(definition[1], ",")
			conclusionStrings := strings.Split(definition[2], ",")

			conclusions, err := ParseConclusions(conclusionStrings)
			if err != nil {
				return nil, err
			}

			rule, err := NewFilterRule(filterType, values, conclusions)
			if err != nil {
				return nil, err
			}

			used = append(used, filterType)
			whitelists = append(whitelists, *rule)
		}
	}

	filter := NewFilter(whitelists)

	return &filter, nil
}
