package internal_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/mcp"
	"gopkg.in/yaml.v3"
)

const maxSkillBytes = 8192

var (
	semverRe      = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	resumaiToolRe = regexp.MustCompile(`\bresumai_[a-z0-9_]+\b`)
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{".", ".."} {
		if _, err := os.Stat(filepath.Join(candidate, "plugin.json")); err == nil {
			abs, absErr := filepath.Abs(candidate)
			if absErr != nil {
				t.Fatalf("abs path for %s: %v", candidate, absErr)
			}
			return abs
		}
	}
	t.Fatalf("could not locate repository root containing plugin.json")
	return ""
}

type pluginManifest struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	Version          string   `json:"version"`
	Disabled         *bool    `json:"disabled"`
	Logo             string   `json:"logo"`
	Description      string   `json:"description"`
	SuggestedPrompts []string `json:"suggestedPrompts"`
}

func validatePluginManifest(data []byte, rootDir string) error {
	var rawKeys map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawKeys); err != nil {
		return fmt.Errorf("parse plugin.json: %w", err)
	}
	allowedKeys := map[string]bool{
		"name":             true,
		"displayName":      true,
		"version":          true,
		"disabled":         true,
		"logo":             true,
		"description":      true,
		"suggestedPrompts": true,
	}
	for k := range rawKeys {
		if !allowedKeys[k] {
			return fmt.Errorf("disallowed top-level key %q in plugin.json", k)
		}
	}

	var m pluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("decode plugin.json: %w", err)
	}
	if m.Name != "resumai" {
		return fmt.Errorf("plugin name must be %q, got %q", "resumai", m.Name)
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		return errors.New("displayName must be non-empty")
	}
	if !semverRe.MatchString(m.Version) {
		return fmt.Errorf("version %q is not valid semver", m.Version)
	}
	if m.Disabled == nil || *m.Disabled {
		return errors.New("disabled must be explicitly set to false")
	}
	if strings.TrimSpace(m.Description) == "" {
		return errors.New("description must be non-empty")
	}
	if m.Logo != "assets/logo.svg" {
		return fmt.Errorf("logo must be %q, got %q", "assets/logo.svg", m.Logo)
	}
	logoPath := filepath.Join(rootDir, filepath.FromSlash(m.Logo))
	info, err := os.Stat(logoPath)
	if err != nil || info.IsDir() {
		return fmt.Errorf("logo file %q does not exist", logoPath)
	}
	if len(m.SuggestedPrompts) == 0 || len(m.SuggestedPrompts) > 3 {
		return fmt.Errorf("suggestedPrompts must have 1..3 entries, got %d", len(m.SuggestedPrompts))
	}
	for i, prompt := range m.SuggestedPrompts {
		if strings.TrimSpace(prompt) == "" {
			return fmt.Errorf("suggestedPrompts[%d] must be non-empty", i)
		}
	}
	return nil
}

func validateZeroContextBloat(rootDir string) error {
	forbiddenPaths := []string{
		filepath.Join(rootDir, "rules"),
		filepath.Join(rootDir, "AGENTS.md"),
		filepath.Join(rootDir, "rules", "AGENTS.md"),
	}
	for _, p := range forbiddenPaths {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("pillar 5 violation: forbidden always-on context artifact %q exists", p)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %q: %w", p, err)
		}
	}
	return nil
}

type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

type mcpConfigFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

func validateMCPConfig(data []byte) error {
	var rawTop map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawTop); err != nil {
		return fmt.Errorf("parse mcp_config.json: %w", err)
	}
	for k := range rawTop {
		if k != "mcpServers" {
			return fmt.Errorf("disallowed top-level key %q in mcp_config.json", k)
		}
	}

	var cfg mcpConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("decode mcp_config.json: %w", err)
	}
	entry, ok := cfg.MCPServers["resumai"]
	if !ok {
		return errors.New("mcpServers missing required \"resumai\" server")
	}
	if len(cfg.MCPServers) != 1 {
		return fmt.Errorf("expected exactly 1 mcpServers entry, got %d", len(cfg.MCPServers))
	}
	if entry.Command != "resumai" {
		return fmt.Errorf("mcpServers.resumai.command must be OS-agnostic %q, got %q", "resumai", entry.Command)
	}
	if strings.ContainsAny(entry.Command, `/\:.`) {
		return fmt.Errorf("mcpServers.resumai.command %q must not contain OS paths or extensions", entry.Command)
	}
	if len(entry.Args) != 1 || entry.Args[0] != "mcp" {
		return fmt.Errorf("mcpServers.resumai.args must be [\"mcp\"], got %v", entry.Args)
	}
	return nil
}

func splitYAMLFrontmatter(content []byte) ([]byte, []byte, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return nil, nil, errors.New("missing opening YAML frontmatter delimiter '---'")
	}
	rest := normalized[4:]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return nil, nil, errors.New("missing closing YAML frontmatter delimiter '---'")
	}
	return rest[:idx], rest[idx+5:], nil
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func validateSkillFile(expectedName string, content []byte) error {
	if len(content) > maxSkillBytes {
		return fmt.Errorf("skill %q exceeds %d-byte cap (%d bytes)", expectedName, maxSkillBytes, len(content))
	}
	fmBytes, body, err := splitYAMLFrontmatter(content)
	if err != nil {
		return fmt.Errorf("skill %q frontmatter: %w", expectedName, err)
	}
	var fm skillFrontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return fmt.Errorf("skill %q YAML decode: %w", expectedName, err)
	}
	if fm.Name != expectedName {
		return fmt.Errorf("skill frontmatter name %q does not match directory %q", fm.Name, expectedName)
	}
	if strings.TrimSpace(fm.Description) == "" {
		return fmt.Errorf("skill %q description must be non-empty", expectedName)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return fmt.Errorf("skill %q body must be non-empty", expectedName)
	}
	return nil
}

type agentFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Subagent    bool     `yaml:"subagent"`
	InheritMCP  bool     `yaml:"inheritMcp"`
	Tools       []string `yaml:"tools"`
}

func validateCriticAgent(expectedName string, content []byte) error {
	fmBytes, body, err := splitYAMLFrontmatter(content)
	if err != nil {
		return fmt.Errorf("agent %q frontmatter: %w", expectedName, err)
	}
	var fm agentFrontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return fmt.Errorf("agent %q YAML decode: %w", expectedName, err)
	}
	if fm.Name != expectedName {
		return fmt.Errorf("agent name must be %q, got %q", expectedName, fm.Name)
	}
	if strings.TrimSpace(fm.Description) == "" {
		return fmt.Errorf("agent %q description must be non-empty", expectedName)
	}
	if fm.Model != "flash" {
		return fmt.Errorf("agent %q model must be \"flash\", got %q", expectedName, fm.Model)
	}
	if !fm.Subagent {
		return fmt.Errorf("agent %q must declare subagent: true", expectedName)
	}
	if !fm.InheritMCP {
		return fmt.Errorf("agent %q must declare inheritMcp: true", expectedName)
	}
	if len(fm.Tools) == 0 {
		return fmt.Errorf("agent %q must declare non-empty read-only tools", expectedName)
	}
	readOnlyAllowlist := map[string]bool{
		"read_file":    true,
		"send_message": true,
	}
	for _, tool := range fm.Tools {
		if !readOnlyAllowlist[tool] {
			return fmt.Errorf("agent %q declares disallowed or mutating tool %q", expectedName, tool)
		}
	}
	if !bytes.Contains(body, []byte("resumai_get_audit_bundle")) {
		return fmt.Errorf("agent %q must instruct calling resumai_get_audit_bundle", expectedName)
	}
	return nil
}

func validateToolReferences(docs map[string][]byte, registered map[string]struct{}) ([]string, error) {
	seen := make(map[string]struct{})
	for docName, content := range docs {
		matches := resumaiToolRe.FindAllString(string(content), -1)
		for _, tok := range matches {
			if _, ok := registered[tok]; !ok {
				return nil, fmt.Errorf("%s references unknown resumai_* MCP tool %q", docName, tok)
			}
			seen[tok] = struct{}{}
		}
	}
	var missing []string
	for toolName := range registered {
		if _, ok := seen[toolName]; !ok {
			missing = append(missing, toolName)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return missing, fmt.Errorf("registered MCP tools missing from plugin docs/skills/agents: %v", missing)
	}
	return nil, nil
}

func TestLivePluginBundleContracts(t *testing.T) {
	root := findRepoRoot(t)

	manifestBytes, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	if err := validatePluginManifest(manifestBytes, root); err != nil {
		t.Fatalf("plugin.json validation failed: %v", err)
	}
	if err := validateZeroContextBloat(root); err != nil {
		t.Fatalf("zero context bloat validation failed: %v", err)
	}

	mcpBytes, err := os.ReadFile(filepath.Join(root, "mcp_config.json"))
	if err != nil {
		t.Fatalf("read mcp_config.json: %v", err)
	}
	if err := validateMCPConfig(mcpBytes); err != nil {
		t.Fatalf("mcp_config.json validation failed: %v", err)
	}

	// Verify all 4 shipped skills exist, parse cleanly, and fit under 8192 bytes.
	expectedSkills := []string{
		"career-archaeology",
		"opportunity-triage",
		"resume-tailor",
		"vault-portability",
	}
	docs := make(map[string][]byte)
	for _, skillName := range expectedSkills {
		p := filepath.Join(root, "skills", skillName, "SKILL.md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read skill %s: %v", skillName, err)
		}
		if err := validateSkillFile(skillName, b); err != nil {
			t.Fatalf("validate skill %s: %v", skillName, err)
		}
		docs["skills/"+skillName+"/SKILL.md"] = b
	}

	// Verify resume-tailor explicitly documents Model: "flash" and nested draft keys.
	tailorBody := string(docs["skills/resume-tailor/SKILL.md"])
	for _, requiredSnippet := range []string{`Model: "flash"`, "role_title", "organization", "highlights"} {
		if !strings.Contains(tailorBody, requiredSnippet) {
			t.Errorf("skills/resume-tailor/SKILL.md missing required guidance %q", requiredSnippet)
		}
	}

	// Verify both clean-room critic subagents.
	expectedCritics := []string{
		domain.CriticFactChecker,
		domain.CriticAlignmentChecker,
	}
	for _, critic := range expectedCritics {
		p := filepath.Join(root, "agents", critic+".md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read agent %s: %v", critic, err)
		}
		if err := validateCriticAgent(critic, b); err != nil {
			t.Fatalf("validate agent %s: %v", critic, err)
		}
		docs["agents/"+critic+".md"] = b
	}

	readmeBytes, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	docs["README.md"] = readmeBytes

	// Verify 1:1 parity between registered MCP tools (24) and plugin markdown references.
	srv := mcp.NewServer(nil)
	tools := srv.ListTools()
	if len(tools) != 24 {
		t.Fatalf("expected 24 registered MCP tools, got %d", len(tools))
	}
	registered := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		registered[tool.Name] = struct{}{}
	}
	if _, err := validateToolReferences(docs, registered); err != nil {
		t.Fatalf("MCP tool reference parity failed: %v", err)
	}

	// Verify release workflow safeguards (exact version check, plugin.json parity, semver check, --prerelease).
	releaseBytes, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yaml"))
	if err != nil {
		t.Fatalf("read release.yaml: %v", err)
	}
	releaseText := string(releaseBytes)
	for _, required := range []string{
		`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`,
		`jq -er '.version' plugin.json`,
		`"resumai version ${VERSION}"`,
		`--prerelease`,
	} {
		if !strings.Contains(releaseText, required) {
			t.Errorf(".github/workflows/release.yaml missing safeguard %q", required)
		}
	}
}

func TestPluginBundleValidators_TableDriven(t *testing.T) {
	t.Run("plugin_manifest", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmp, "assets"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "assets", "logo.svg"), []byte("<svg/>"), 0o644); err != nil {
			t.Fatal(err)
		}

		cases := []struct {
			name    string
			json    string
			wantErr bool
		}{
			{
				name: "valid_manifest",
				json: `{"name":"resumai","displayName":"ResumAI","version":"0.2.0","disabled":false,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["One"]}`,
			},
			{
				name:    "disabled_true_rejected",
				json:    `{"name":"resumai","displayName":"ResumAI","version":"0.2.0","disabled":true,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["One"]}`,
				wantErr: true,
			},
			{
				name:    "unknown_top_level_key_rejected",
				json:    `{"name":"resumai","displayName":"ResumAI","version":"0.2.0","disabled":false,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["One"],"extraKey":1}`,
				wantErr: true,
			},
			{
				name:    "invalid_semver_rejected",
				json:    `{"name":"resumai","displayName":"ResumAI","version":"v0.2","disabled":false,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["One"]}`,
				wantErr: true,
			},
			{
				name:    "too_many_suggested_prompts_rejected",
				json:    `{"name":"resumai","displayName":"ResumAI","version":"0.2.0","disabled":false,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["1","2","3","4"]}`,
				wantErr: true,
			},
			{
				name:    "empty_suggested_prompt_rejected",
				json:    `{"name":"resumai","displayName":"ResumAI","version":"0.2.0","disabled":false,"logo":"assets/logo.svg","description":"Desc","suggestedPrompts":["  "]}`,
				wantErr: true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := validatePluginManifest([]byte(tc.json), tmp)
				if (err != nil) != tc.wantErr {
					t.Fatalf("validatePluginManifest() err=%v, wantErr=%v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("zero_context_bloat", func(t *testing.T) {
		cleanDir := t.TempDir()
		if err := validateZeroContextBloat(cleanDir); err != nil {
			t.Fatalf("expected clean dir to pass, got %v", err)
		}

		withRules := t.TempDir()
		if err := os.MkdirAll(filepath.Join(withRules, "rules"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := validateZeroContextBloat(withRules); err == nil {
			t.Fatalf("expected rules/ directory to be rejected")
		}

		withAgentsMD := t.TempDir()
		if err := os.WriteFile(filepath.Join(withAgentsMD, "AGENTS.md"), []byte("# rules"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := validateZeroContextBloat(withAgentsMD); err == nil {
			t.Fatalf("expected AGENTS.md to be rejected")
		}
	})

	t.Run("mcp_config", func(t *testing.T) {
		cases := []struct {
			name    string
			json    string
			wantErr bool
		}{
			{name: "valid_os_agnostic", json: `{"mcpServers":{"resumai":{"command":"resumai","args":["mcp"]}}}`},
			{name: "windows_exe_rejected", json: `{"mcpServers":{"resumai":{"command":"resumai.exe","args":["mcp"]}}}`, wantErr: true},
			{name: "absolute_path_rejected", json: `{"mcpServers":{"resumai":{"command":"/usr/local/bin/resumai","args":["mcp"]}}}`, wantErr: true},
			{name: "wrong_args_rejected", json: `{"mcpServers":{"resumai":{"command":"resumai","args":["serve"]}}}`, wantErr: true},
			{name: "unknown_top_key_rejected", json: `{"mcpServers":{"resumai":{"command":"resumai","args":["mcp"]}},"extra":1}`, wantErr: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := validateMCPConfig([]byte(tc.json))
				if (err != nil) != tc.wantErr {
					t.Fatalf("validateMCPConfig() err=%v, wantErr=%v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("skill_size_boundary_and_frontmatter", func(t *testing.T) {
		header := "---\nname: sample-skill\ndescription: sample\n---\n"
		exactPad := strings.Repeat("a", maxSkillBytes-len(header))
		if err := validateSkillFile("sample-skill", []byte(header+exactPad)); err != nil {
			t.Fatalf("expected exact 8192-byte skill to pass, got %v", err)
		}
		overPad := strings.Repeat("a", maxSkillBytes-len(header)+1)
		if err := validateSkillFile("sample-skill", []byte(header+overPad)); err == nil {
			t.Fatalf("expected 8193-byte skill to fail")
		}
		if err := validateSkillFile("other-skill", []byte(header+"ok")); err == nil {
			t.Fatalf("expected mismatched skill name to fail")
		}
	})

	t.Run("critic_agent_frontmatter", func(t *testing.T) {
		cases := []struct {
			name    string
			content string
			wantErr bool
		}{
			{
				name:    "valid_read_only_critic",
				content: "---\nname: resume-fact-checker\ndescription: critic\nmodel: flash\nsubagent: true\ninheritMcp: true\ntools:\n  - read_file\n  - send_message\n---\nCall resumai_get_audit_bundle.\n",
			},
			{
				name:    "write_file_tool_rejected",
				content: "---\nname: resume-fact-checker\ndescription: critic\nmodel: flash\nsubagent: true\ninheritMcp: true\ntools:\n  - read_file\n  - write_file\n---\nCall resumai_get_audit_bundle.\n",
				wantErr: true,
			},
			{
				name:    "run_command_tool_rejected",
				content: "---\nname: resume-fact-checker\ndescription: critic\nmodel: flash\nsubagent: true\ninheritMcp: true\ntools:\n  - run_command\n---\nCall resumai_get_audit_bundle.\n",
				wantErr: true,
			},
			{
				name:    "non_flash_model_rejected",
				content: "---\nname: resume-fact-checker\ndescription: critic\nmodel: inherit\nsubagent: true\ninheritMcp: true\ntools:\n  - read_file\n---\nCall resumai_get_audit_bundle.\n",
				wantErr: true,
			},
			{
				name:    "inherit_mcp_false_rejected",
				content: "---\nname: resume-fact-checker\ndescription: critic\nmodel: flash\nsubagent: true\ninheritMcp: false\ntools:\n  - read_file\n---\nCall resumai_get_audit_bundle.\n",
				wantErr: true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := validateCriticAgent(domain.CriticFactChecker, []byte(tc.content))
				if (err != nil) != tc.wantErr {
					t.Fatalf("validateCriticAgent() err=%v, wantErr=%v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("tool_reference_parity", func(t *testing.T) {
		registered := map[string]struct{}{
			"resumai_vault_status":     {},
			"resumai_get_audit_bundle": {},
		}
		if _, err := validateToolReferences(map[string][]byte{
			"README.md": []byte("Use resumai_vault_status and resumai_get_audit_bundle."),
		}, registered); err != nil {
			t.Fatalf("expected valid parity, got %v", err)
		}
		if _, err := validateToolReferences(map[string][]byte{
			"README.md": []byte("Use resumai_vault_status, resumai_get_audit_bundle, and resumai_bogus_tool."),
		}, registered); err == nil {
			t.Fatalf("expected unknown tool resumai_bogus_tool to fail")
		}
		if _, err := validateToolReferences(map[string][]byte{
			"README.md": []byte("Use resumai_vault_status only."),
		}, registered); err == nil {
			t.Fatalf("expected missing registered tool to fail parity")
		}
	})
}
