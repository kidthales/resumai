package gate0

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKindGuards_DirectAST(t *testing.T) {
	t.Parallel()

	nonScalarKey := &yaml.Node{Kind: yaml.SequenceNode, Line: 4}
	jobMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "runs-on"},
			{Kind: yaml.ScalarNode, Value: "ubuntu-latest"},
		},
	}
	root := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "jobs"},
			{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					nonScalarKey,
					jobMapping,
				},
			},
		},
	}

	violations := inspectWorkflowRoot("direct.yaml", root)
	if len(violations) != 1 || !strings.Contains(violations[0], "job key must be a scalar string") {
		t.Fatalf("expected non-scalar job key violation, got %v", violations)
	}

	if _, ok := mappingLookup(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			nonScalarKey,
			{Kind: yaml.ScalarNode, Value: "ignored"},
		},
	}, ""); ok {
		t.Fatalf("expected mappingLookup to ignore non-scalar keys")
	}
}
