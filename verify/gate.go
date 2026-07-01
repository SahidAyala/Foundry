package verify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"foundry/domain"
)

// Gate evaluates a set of Validators and produces a verdict.
type Gate struct {
	validators []*Validator
	rule       string // "all-pass" only, for M0
}

// NewGate creates a Gate that evaluates validators according to rule.
// M0 supports only the "all-pass" rule: every validator must pass.
func NewGate(rule string, validators ...*Validator) (*Gate, error) {
	if rule != "all-pass" {
		return nil, fmt.Errorf("verify: unsupported gate rule %q", rule)
	}
	if len(validators) == 0 {
		return nil, errors.New("verify: gate requires at least one validator")
	}
	return &Gate{validators: validators, rule: rule}, nil
}

// Evaluate runs every validator against workspace and returns the resulting Judgment.
func (g *Gate) Evaluate(ctx context.Context, workspace string) (*domain.Judgment, error) {
	checked := make([]string, 0, len(g.validators))
	allPassed := true

	for _, v := range g.validators {
		result, err := v.Run(ctx, workspace)
		if err != nil {
			return nil, fmt.Errorf("verify: evaluate %q: %w", v.Name, err)
		}
		checked = append(checked, formatChecked(result))
		if !result.Passed {
			allPassed = false
		}
	}

	verdict := "fail"
	if allPassed {
		verdict = "pass"
	}

	return &domain.Judgment{
		Verdict: verdict,
		Checked: checked,
	}, nil
}

func formatChecked(r *Result) string {
	status := "pass"
	if !r.Passed {
		status = "fail"
	}
	output := strings.TrimRight(r.Output, "\n")
	if output == "" {
		return fmt.Sprintf("%s: %s", r.Name, status)
	}
	return fmt.Sprintf("%s: %s\n%s", r.Name, status, output)
}
