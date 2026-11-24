// Package gate0 implements Gate 0 workflow verification against GitHub Actions YAML ASTs.
package gate0

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type allowlistedStepRule struct {
	requireExactRun bool
	expectedRun     string
}

// exactStepAllowlist maps "jobID/stepName" pairs to an allowlist rule specifying
// either an exact expected run command or unconstrained command text.
var exactStepAllowlist = map[string]allowlistedStepRule{
	"test-and-build/Install govulncheck":                         {requireExactRun: true, expectedRun: "go install golang.org/x/vuln/cmd/govulncheck@v1.9.0"},
	"notify/Dispatch consolidated status card to #dev-ci-builds": {},
	"release/Build cross-platform stripped release binaries":     {},
	"release/Verify tagged commit is on main":                    {},
	"release/Generate SHA256SUMS":                                {},
	"release/Verify resumai version output":                      {},
	"release/Attach release assets":                              {},
}

var allowedCanonicalStepKeys = map[string]bool{
	"id":                true,
	"name":              true,
	"run":               true,
	"shell":             true,
	"continue-on-error": true,
	"if":                true,
	"working-directory": true,
	"env":               true,
	"timeout-minutes":   true,
}

var canonicalCIRuns = map[string]bool{
	"./scripts/ci-local.ps1 -CI":            true,
	"pwsh ./scripts/ci-local.ps1 -CI":       true,
	"pwsh -File ./scripts/ci-local.ps1 -CI": true,
}

var namedVerifyJobs = map[string]bool{
	"test-and-build": true,
	"verify":         true,
}

var reusableVerifyWorkflows = map[string]bool{
	"./.github/workflows/ci.yml":  true,
	"./.github/workflows/ci.yaml": true,
}

// VerifyWorkflowsDir validates every *.yml and *.yaml file in dir against the Gate 0
// single-source CI contract. Fatal filesystem/empty-dir failures return an error;
// workflow rule and YAML structural violations are returned as a slice.
func VerifyWorkflowsDir(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("Workflows directory not found: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read workflows directory %s: %w", dir, err)
	}

	var wfFiles []string
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		fInfo, statErr := os.Stat(fullPath)
		if statErr != nil || fInfo.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".yml" || ext == ".yaml" {
			wfFiles = append(wfFiles, entry.Name())
		}
	}
	sort.Strings(wfFiles)

	if len(wfFiles) == 0 {
		return nil, fmt.Errorf("Gate 0 failed: no workflow files (*.yml, *.yaml) found in %s", dir)
	}

	violations := []string{}
	for _, name := range wfFiles {
		fullPath := filepath.Join(dir, name)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			violations = append(violations, fmt.Sprintf("%s: failed to read workflow file: %v", name, readErr))
			continue
		}
		violations = append(violations, VerifyWorkflowYAML(name, data)...)
	}

	return violations, nil
}

// VerifyWorkflowYAML validates a single GitHub Actions workflow YAML payload.
func VerifyWorkflowYAML(fileName string, data []byte) []string {
	root, structViolations := decodeStrictWorkflowRoot(fileName, data)
	if len(structViolations) > 0 {
		return structViolations
	}
	return inspectWorkflowRoot(fileName, root)
}

func decodeStrictWorkflowRoot(fileName string, data []byte) (*yaml.Node, []string) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, []string{fmt.Sprintf("%s: workflow YAML is empty.", fileName)}
		}
		return nil, []string{fmt.Sprintf("%s: invalid YAML: %v", fileName, err)}
	}

	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, []string{fmt.Sprintf("%s: multiple YAML documents are prohibited to ensure deterministic Gate 0 auditing.", fileName)}
	}

	root := &doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) != 1 {
			return nil, []string{fmt.Sprintf("%s: workflow YAML document is empty.", fileName)}
		}
		root = root.Content[0]
	}

	if root.Kind != yaml.MappingNode {
		return nil, []string{fmt.Sprintf("%s: workflow root must be a YAML mapping.", fileName)}
	}

	var astViolations []string
	validateYAMLNodeAST(fileName, root, &astViolations)
	if len(astViolations) > 0 {
		return nil, astViolations
	}

	return root, nil
}

// validateYAMLNodeAST fails closed on YAML anchors, aliases, merge keys, non-scalar keys,
// and duplicate mapping keys so no construct can silently shadow or alias overrides.
func validateYAMLNodeAST(fileName string, node *yaml.Node, violations *[]string) {
	if node == nil {
		return
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		*violations = append(*violations, fmt.Sprintf("%s: YAML anchors and aliases are prohibited to ensure deterministic Gate 0 auditing (line %d).", fileName, node.Line))
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content)%2 != 0 {
			*violations = append(*violations, fmt.Sprintf("%s: malformed YAML mapping (line %d).", fileName, node.Line))
			return
		}
		seenKeys := make(map[string]int, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			kNode := node.Content[i]
			vNode := node.Content[i+1]

			if kNode.Anchor != "" || kNode.Kind == yaml.AliasNode {
				*violations = append(*violations, fmt.Sprintf("%s: YAML anchors and aliases are prohibited to ensure deterministic Gate 0 auditing (line %d).", fileName, kNode.Line))
				continue
			}
			if kNode.Kind != yaml.ScalarNode {
				*violations = append(*violations, fmt.Sprintf("%s: non-scalar YAML mapping keys are prohibited (line %d).", fileName, kNode.Line))
				continue
			}
			if kNode.Tag == "!!merge" || kNode.Value == "<<" {
				*violations = append(*violations, fmt.Sprintf("%s: YAML merge keys ('<<') are prohibited to ensure deterministic Gate 0 auditing (line %d).", fileName, kNode.Line))
				continue
			}
			if prevLine, dup := seenKeys[kNode.Value]; dup {
				*violations = append(*violations, fmt.Sprintf("%s: duplicate YAML mapping key %q (lines %d and %d) is prohibited.", fileName, kNode.Value, prevLine, kNode.Line))
			} else {
				seenKeys[kNode.Value] = kNode.Line
			}
			validateYAMLNodeAST(fileName, vNode, violations)
		}
	case yaml.SequenceNode:
		for _, elem := range node.Content {
			validateYAMLNodeAST(fileName, elem, violations)
		}
	}
}

func inspectWorkflowRoot(fileName string, root *yaml.Node) []string {
	var violations []string
	var verifyJobNames []string

	jobsNode, hasJobs := mappingLookup(root, "jobs")
	if hasJobs {
		if isNullScalar(jobsNode) {
			// Empty jobs mapping treated as having zero jobs.
		} else if jobsNode.Kind != yaml.MappingNode {
			violations = append(violations, fmt.Sprintf("%s: top-level 'jobs' must be a YAML mapping.", fileName))
		} else {
			for i := 0; i+1 < len(jobsNode.Content); i += 2 {
				keyNode := jobsNode.Content[i]
				if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
					line := 0
					if keyNode != nil {
						line = keyNode.Line
					}
					violations = append(violations, fmt.Sprintf("%s: job key must be a scalar string (line %d).", fileName, line))
					continue
				}
				jobID := keyNode.Value
				jobNode := jobsNode.Content[i+1]
				jobVerify, jobViolations := inspectJob(fileName, jobID, jobNode)
				if jobVerify {
					verifyJobNames = append(verifyJobNames, jobID)
				}
				violations = append(violations, jobViolations...)
			}
		}
	}

	if len(verifyJobNames) > 0 {
		verifyList := strings.Join(verifyJobNames, ", ")
		if wfDefaultsNode, ok := mappingLookup(root, "defaults"); ok {
			violations = append(violations, inspectDefaults(fileName, "workflow", fmt.Sprintf(" on verify job(s) (%s)", verifyList), wfDefaultsNode)...)
		}
		if _, hasWfEnv := mappingLookup(root, "env"); hasWfEnv {
			violations = append(violations, fmt.Sprintf("%s: workflow must not define 'env:' override on verify job(s) (%s) to preserve single-source contract.", fileName, verifyList))
		}
	}

	return violations
}

func inspectJob(fileName, jobID string, jobNode *yaml.Node) (bool, []string) {
	if jobNode.Kind != yaml.MappingNode {
		return namedVerifyJobs[jobID], []string{fmt.Sprintf("%s: job '%s' must be a YAML mapping.", fileName, jobID)}
	}

	var violations []string
	jobRunsCanonicalCI := false
	jobHasSteps := false
	jobUsesReusable := ""

	if usesNode, ok := mappingLookup(jobNode, "uses"); ok {
		val, err := scalarString(usesNode)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: job '%s' 'uses:' must be a scalar string.", fileName, jobID))
		} else {
			jobUsesReusable = strings.TrimSpace(val)
		}
	}

	if stepsNode, hasSteps := mappingLookup(jobNode, "steps"); hasSteps {
		if stepsNode.Kind != yaml.SequenceNode {
			violations = append(violations, fmt.Sprintf("%s: job '%s' 'steps' must be a YAML sequence.", fileName, jobID))
		} else {
			jobHasSteps = true
			for _, stepNode := range stepsNode.Content {
				stepCanonical, stepViolations := inspectStep(fileName, jobID, stepNode)
				if stepCanonical {
					jobRunsCanonicalCI = true
				}
				violations = append(violations, stepViolations...)
			}
		}
	}

	isNamedVerify := namedVerifyJobs[jobID]
	isVerifyJob := isNamedVerify || jobRunsCanonicalCI || reusableVerifyWorkflows[jobUsesReusable]

	if isNamedVerify {
		if jobHasSteps && !jobRunsCanonicalCI {
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' defines steps but does not invoke './scripts/ci-local.ps1 -CI'.", fileName, jobID))
		} else if !jobHasSteps && !reusableVerifyWorkflows[jobUsesReusable] {
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must either invoke './scripts/ci-local.ps1 -CI' or call './.github/workflows/ci.yaml' (got uses: '%s').", fileName, jobID, jobUsesReusable))
		}
	}

	if isVerifyJob {
		if jobHasSteps && jobUsesReusable != "" {
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define both 'uses:' and 'steps:'.", fileName, jobID))
		}
		if jobDefaultsNode, ok := mappingLookup(jobNode, "defaults"); ok {
			violations = append(violations, inspectDefaults(fileName, fmt.Sprintf("verify job '%s'", jobID), "", jobDefaultsNode)...)
		}
		if wdNode, hasDirectWD := mappingLookup(jobNode, "working-directory"); hasDirectWD {
			wdVal := scalarOrPlaceholder(wdNode)
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define job-level 'working-directory:' ('%s') to preserve single-source contract.", fileName, jobID, wdVal))
		}
		if _, hasJobEnv := mappingLookup(jobNode, "env"); hasJobEnv {
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define job-level 'env:' override to preserve single-source contract.", fileName, jobID))
		}
		if ifNode, hasIf := mappingLookup(jobNode, "if"); hasIf {
			ifVal := scalarOrPlaceholder(ifNode)
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define job-level 'if:' conditional ('%s') to preserve single-source contract.", fileName, jobID, ifVal))
		}
		if coeNode, hasCOE := mappingLookup(jobNode, "continue-on-error"); hasCOE {
			if isTruthyOrDynamicFlag(coeNode) {
				violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define job-level 'continue-on-error: true' to preserve single-source contract.", fileName, jobID))
			}
		}
		if timeoutNode, hasTimeout := mappingLookup(jobNode, "timeout-minutes"); hasTimeout {
			timeoutVal := scalarOrPlaceholder(timeoutNode)
			violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not define job-level 'timeout-minutes:' ('%s') to preserve single-source contract.", fileName, jobID, timeoutVal))
		}
		if stratNode, hasStrat := mappingLookup(jobNode, "strategy"); hasStrat {
			if !isNullScalar(stratNode) {
				if stratNode.Kind != yaml.MappingNode {
					violations = append(violations, fmt.Sprintf("%s: verify job '%s' 'strategy' must be a YAML mapping.", fileName, jobID))
				} else if ffNode, hasFF := mappingLookup(stratNode, "fail-fast"); hasFF {
					if ffNode.Kind != yaml.ScalarNode {
						violations = append(violations, fmt.Sprintf("%s: verify job '%s' 'strategy.fail-fast' must be a scalar boolean.", fileName, jobID))
					} else if isTruthyOrDynamicFlag(ffNode) {
						val := strings.TrimSpace(ffNode.Value)
						if strings.EqualFold(val, "true") {
							violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not set 'strategy.fail-fast: true' to prevent matrix cancellation.", fileName, jobID))
						} else {
							violations = append(violations, fmt.Sprintf("%s: verify job '%s' must not set 'strategy.fail-fast' to dynamic or non-false value ('%s') to prevent matrix cancellation.", fileName, jobID, val))
						}
					}
				}
			}
		}
	}

	return isVerifyJob, violations
}

func inspectStep(fileName, jobID string, stepNode *yaml.Node) (bool, []string) {
	if stepNode.Kind != yaml.MappingNode {
		return false, []string{fmt.Sprintf("%s: step in job '%s' must be a YAML mapping (line %d).", fileName, jobID, stepNode.Line)}
	}

	var violations []string
	stepName := ""
	if nameNode, ok := mappingLookup(stepNode, "name"); ok {
		if nameNode.Kind != yaml.ScalarNode {
			violations = append(violations, fmt.Sprintf("%s: step 'name' in job '%s' must be a scalar string (line %d).", fileName, jobID, nameNode.Line))
		} else {
			stepName = strings.TrimSpace(nameNode.Value)
		}
	}

	runNode, hasRun := mappingLookup(stepNode, "run")
	if !hasRun {
		return false, violations
	}
	if runNode.Kind != yaml.ScalarNode {
		violations = append(violations, fmt.Sprintf("%s: job '%s' step '%s' 'run:' must be a scalar string.", fileName, jobID, stepName))
		return false, violations
	}

	runText := normalizeRunText(runNode.Value)
	pairKey := jobID + "/" + stepName
	isCanonicalCI := canonicalCIRuns[runText]
	rule, isAllowlisted := exactStepAllowlist[pairKey]

	if isCanonicalCI {
		if ifNode, hasIf := mappingLookup(stepNode, "if"); hasIf {
			ifVal := scalarOrPlaceholder(ifNode)
			violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'if:' conditional ('%s') to preserve single-source contract.", fileName, stepName, jobID, ifVal))
		}
		if coeNode, hasCOE := mappingLookup(stepNode, "continue-on-error"); hasCOE && isTruthyOrDynamicFlag(coeNode) {
			violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'continue-on-error: true' to preserve single-source contract.", fileName, stepName, jobID))
		}
		if wdNode, hasWD := mappingLookup(stepNode, "working-directory"); hasWD {
			wdVal := scalarOrPlaceholder(wdNode)
			violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'working-directory:' ('%s') to preserve single-source contract.", fileName, stepName, jobID, wdVal))
		}
		if _, hasEnv := mappingLookup(stepNode, "env"); hasEnv {
			violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'env:' override to preserve single-source contract.", fileName, stepName, jobID))
		}
		if shellNode, hasShell := mappingLookup(stepNode, "shell"); hasShell {
			shellVal, err := scalarString(shellNode)
			if err != nil || strings.TrimSpace(shellVal) != "pwsh" {
				violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'shell:' override ('%s') (only 'pwsh' is permitted) to preserve single-source contract.", fileName, stepName, jobID, scalarOrPlaceholder(shellNode)))
			}
		}
		if timeoutNode, hasTimeout := mappingLookup(stepNode, "timeout-minutes"); hasTimeout {
			timeoutVal := scalarOrPlaceholder(timeoutNode)
			violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define 'timeout-minutes:' ('%s') to preserve single-source contract.", fileName, stepName, jobID, timeoutVal))
		}
		for i := 0; i+1 < len(stepNode.Content); i += 2 {
			kNode := stepNode.Content[i]
			if kNode == nil || kNode.Kind != yaml.ScalarNode {
				continue
			}
			if !allowedCanonicalStepKeys[kNode.Value] {
				violations = append(violations, fmt.Sprintf("%s: canonical CI step '%s' in job '%s' must not define execution-altering or unsupported key '%s:' to preserve single-source contract.", fileName, stepName, jobID, kNode.Value))
			}
		}
	}

	if isAllowlisted {
		if rule.requireExactRun && runText != rule.expectedRun {
			violations = append(violations, fmt.Sprintf("%s: job '%s' step '%s' run script ('%s') does not match expected allowlisted command ('%s').", fileName, jobID, stepName, runText, rule.expectedRun))
		}
	} else if !isCanonicalCI {
		violations = append(violations, fmt.Sprintf("%s: job '%s' step '%s' runs inline script ('%s') instead of './scripts/ci-local.ps1 -CI' and pair '%s' is not in the Gate 0 exact allowlist.", fileName, jobID, stepName, runText, pairKey))
	}

	return isCanonicalCI, violations
}

func inspectDefaults(fileName, subject, scopeSuffix string, defaultsNode *yaml.Node) []string {
	if defaultsNode == nil || isNullScalar(defaultsNode) {
		return nil
	}
	if defaultsNode.Kind != yaml.MappingNode {
		return []string{fmt.Sprintf("%s: %s defines malformed 'defaults:' (expected YAML mapping)%s.", fileName, subject, scopeSuffix)}
	}

	var violations []string
	for i := 0; i+1 < len(defaultsNode.Content); i += 2 {
		kNode := defaultsNode.Content[i]
		if kNode == nil || kNode.Kind != yaml.ScalarNode {
			continue
		}
		if kNode.Value != "run" {
			violations = append(violations, fmt.Sprintf("%s: %s must not define unsupported 'defaults.%s'%s to preserve single-source contract.", fileName, subject, kNode.Value, scopeSuffix))
		}
	}

	runNode, hasRun := mappingLookup(defaultsNode, "run")
	if !hasRun || isNullScalar(runNode) {
		return violations
	}
	if runNode.Kind != yaml.MappingNode {
		return append(violations, fmt.Sprintf("%s: %s defines malformed 'defaults.run:' (expected YAML mapping)%s.", fileName, subject, scopeSuffix))
	}

	for i := 0; i+1 < len(runNode.Content); i += 2 {
		kNode := runNode.Content[i]
		vNode := runNode.Content[i+1]
		if kNode == nil || kNode.Kind != yaml.ScalarNode {
			continue
		}
		switch kNode.Value {
		case "working-directory":
			violations = append(violations, fmt.Sprintf("%s: %s must not define 'defaults.run.working-directory' ('%s')%s to preserve single-source contract.", fileName, subject, scalarOrPlaceholder(vNode), scopeSuffix))
		case "shell":
			shellVal, err := scalarString(vNode)
			if err != nil || strings.TrimSpace(shellVal) != "pwsh" {
				violations = append(violations, fmt.Sprintf("%s: %s must not define 'defaults.run.shell' ('%s')%s (only 'pwsh' is permitted) to preserve single-source contract.", fileName, subject, scalarOrPlaceholder(vNode), scopeSuffix))
			}
		default:
			violations = append(violations, fmt.Sprintf("%s: %s must not define 'defaults.run.%s'%s to preserve single-source contract.", fileName, subject, kNode.Value, scopeSuffix))
		}
	}

	return violations
}

func mappingLookup(m *yaml.Node, key string) (*yaml.Node, bool) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		kNode := m.Content[i]
		if kNode != nil && kNode.Kind == yaml.ScalarNode && kNode.Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

func isNullScalar(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

func scalarString(n *yaml.Node) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", errors.New("expected scalar node")
	}
	if n.Tag == "!!null" {
		return "", nil
	}
	return n.Value, nil
}

func scalarOrPlaceholder(n *yaml.Node) string {
	s, err := scalarString(n)
	if err != nil {
		return "<non-scalar>"
	}
	return strings.TrimSpace(s)
}

func isTruthyOrDynamicFlag(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.ScalarNode {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(n.Value), "false")
}

func normalizeRunText(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
