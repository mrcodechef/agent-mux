package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkillsSingleSkill(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "---\ndescription: Use Go conventions.\n---\nUse Go conventions.")

	prompt, pathDirs, err := LoadSkills([]string{"go"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if !strings.Contains(prompt, "Available skills") {
		t.Fatalf("prompt missing header: %q", prompt)
	}
	if !strings.Contains(prompt, "- go: Use Go conventions.") {
		t.Fatalf("prompt missing skill reference: %q", prompt)
	}
	if !strings.Contains(prompt, "SKILL.md") {
		t.Fatalf("prompt missing SKILL.md path: %q", prompt)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsNoFrontmatterFallback(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Use Go conventions.\n")

	prompt, _, err := LoadSkills([]string{"go"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if !strings.Contains(prompt, "- go: (no description)") {
		t.Fatalf("prompt = %q, want fallback description", prompt)
	}
}

func TestLoadSkillsMultipleSkillsInOrder(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "---\ndescription: Go only.\n---\nGo only.")
	writeSkillFile(t, cwd, "review", "---\ndescription: Review for regressions.\n---\nReview for regressions.")

	prompt, pathDirs, err := LoadSkills([]string{"go", "review"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	goIdx := strings.Index(prompt, "- go:")
	reviewIdx := strings.Index(prompt, "- review:")
	if goIdx < 0 || reviewIdx < 0 || goIdx >= reviewIdx {
		t.Fatalf("skills not in order: %q", prompt)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsDeduplicatesNames(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Only once.")

	prompt, pathDirs, err := LoadSkills([]string{"go", "go"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if strings.Count(prompt, "- go:") != 1 {
		t.Fatalf("prompt = %q, want single skill reference", prompt)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsWithScriptsDir(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Use helpers.")
	scriptsDir := filepath.Join(cwd, ".claude", "skills", "go", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scripts: %v", err)
	}

	_, pathDirs, err := LoadSkills([]string{"go"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if len(pathDirs) != 1 || pathDirs[0] != scriptsDir {
		t.Fatalf("pathDirs = %#v, want [%q]", pathDirs, scriptsDir)
	}
}

func TestLoadSkillsWithoutScriptsDir(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "No scripts.")

	_, pathDirs, err := LoadSkills([]string{"go"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsNotFoundIncludesSearchedPaths(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Go only.")
	writeSkillFile(t, cwd, "review", "Review only.")

	_, _, err := LoadSkills([]string{"missing"}, cwd, "")
	if err == nil {
		t.Fatal("LoadSkills error = nil, want error")
	}

	msg := err.Error()
	if !strings.Contains(msg, `skill "missing" not found`) {
		t.Fatalf("error = %q, want skill not found message", msg)
	}
	if !strings.Contains(msg, "Searched:") {
		t.Fatalf("error = %q, want searched paths info", msg)
	}
	if !strings.Contains(msg, "Available skills:") {
		t.Fatalf("error = %q, want available skills info", msg)
	}
}

// TestAvailableSkillsExcludesGhostDirs verifies that collectSkills does not include
// directories that are missing SKILL.md in the "Available skills" error message.
func TestAvailableSkillsExcludesGhostDirs(t *testing.T) {
	cwd := t.TempDir()

	// Real skill with SKILL.md.
	writeSkillFile(t, cwd, "real", "Real skill content.")

	// Ghost directory: exists but has no SKILL.md.
	ghostDir := filepath.Join(cwd, ".claude", "skills", "ghost")
	if err := os.MkdirAll(ghostDir, 0o755); err != nil {
		t.Fatalf("MkdirAll ghost: %v", err)
	}

	_, _, err := LoadSkills([]string{"missing"}, cwd, "")
	if err == nil {
		t.Fatal("LoadSkills error = nil, want error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "real") {
		t.Fatalf("error = %q, want real skill listed in Available skills", msg)
	}
	if strings.Contains(msg, "ghost") {
		t.Fatalf("error = %q, ghost dir without SKILL.md should NOT appear in Available skills", msg)
	}
}

func TestLoadSkillsNotFoundIncludesRoleName(t *testing.T) {
	cwd := t.TempDir()

	_, _, err := LoadSkills([]string{"missing"}, cwd, "lifter")
	if err == nil {
		t.Fatal("LoadSkills error = nil, want error")
	}

	msg := err.Error()
	if !strings.Contains(msg, `requested by "lifter"`) {
		t.Fatalf("error = %q, want source name in error", msg)
	}
}

func TestLoadSkillsEmptyNames(t *testing.T) {
	cwd := t.TempDir()

	prompt, pathDirs, err := LoadSkills(nil, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if prompt != "" {
		t.Fatalf("prompt = %q, want empty", prompt)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsRejectsInvalidName(t *testing.T) {
	cwd := t.TempDir()

	_, _, err := LoadSkills([]string{"../bad"}, cwd, "")
	if err == nil {
		t.Fatal("LoadSkills error = nil, want invalid skill name")
	}
	if !strings.Contains(err.Error(), `invalid skill name "../bad"`) {
		t.Fatalf("error = %q, want invalid skill name message", err)
	}
}

func TestLoadSkillsSecondSkillHasScriptsDir(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Go only.")
	writeSkillFile(t, cwd, "review", "Review only.")
	scriptsDir := filepath.Join(cwd, ".claude", "skills", "review", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scripts: %v", err)
	}

	_, pathDirs, err := LoadSkills([]string{"go", "review"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if len(pathDirs) != 1 || pathDirs[0] != scriptsDir {
		t.Fatalf("pathDirs = %#v, want [%q]", pathDirs, scriptsDir)
	}
}

// TestLoadSkillsConfigDirFallback covers Bug 2: a skill defined in the config
// directory (configDir) should be found even when cwd is a completely different
// directory that does not contain a .claude/skills tree.
// TestLoadSkillsEnvSearchPathFallback verifies that AGENT_MUX_SKILL_PATH
// is searched for skills not found in cwd conventions.
func TestLoadSkillsEnvSearchPathFallback(t *testing.T) {
	cwd := t.TempDir()
	searchDir := t.TempDir()

	// Skill lives under searchDir directly (not inside .claude/skills).
	skillDir := filepath.Join(searchDir, "remote-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Remote skill content."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("AGENT_MUX_SKILL_PATH", searchDir)

	prompt, _, err := LoadSkills([]string{"remote-skill"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills with env search_path: %v", err)
	}

	if !strings.Contains(prompt, "- remote-skill:") {
		t.Fatalf("prompt = %q, want remote-skill reference", prompt)
	}
	if !strings.Contains(prompt, "SKILL.md") {
		t.Fatalf("prompt = %q, want SKILL.md path", prompt)
	}
}

// TestLoadSkillsAgentMuxDirConvention verifies that <cwd>/.agent-mux/skills
// is searched.
func TestLoadSkillsAgentMuxDirConvention(t *testing.T) {
	cwd := t.TempDir()
	skillDir := filepath.Join(cwd, ".agent-mux", "skills", "mux-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Mux skill."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prompt, _, err := LoadSkills([]string{"mux-skill"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if !strings.Contains(prompt, "- mux-skill:") {
		t.Fatalf("prompt = %q, want mux-skill reference", prompt)
	}
}

// TestLoadSkillsCwdWinsOverEnvPath verifies cwd takes precedence over env.
func TestLoadSkillsCwdWinsOverEnvPath(t *testing.T) {
	cwd := t.TempDir()
	searchDir := t.TempDir()

	writeSkillFile(t, cwd, "shared", "cwd version")

	skillDir := filepath.Join(searchDir, "shared")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("env version"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("AGENT_MUX_SKILL_PATH", searchDir)

	prompt, _, err := LoadSkills([]string{"shared"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if !strings.Contains(prompt, "- shared:") {
		t.Fatalf("prompt = %q, want shared skill reference", prompt)
	}
	// env is prepended to search order, so env root wins
	if !strings.Contains(prompt, searchDir) {
		t.Fatalf("prompt = %q, want env search path %q to win", prompt, searchDir)
	}
}

// TestLoadSkillsEnvSearchPathWithScriptsDir verifies scripts/ dirs are picked up
// from env-path-resolved skills.
func TestLoadSkillsEnvSearchPathWithScriptsDir(t *testing.T) {
	cwd := t.TempDir()
	searchDir := t.TempDir()

	skillDir := filepath.Join(searchDir, "remote-skill")
	scriptsDir := filepath.Join(skillDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Has scripts."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("AGENT_MUX_SKILL_PATH", searchDir)

	_, pathDirs, err := LoadSkills([]string{"remote-skill"}, cwd, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if len(pathDirs) != 1 || pathDirs[0] != scriptsDir {
		t.Fatalf("pathDirs = %#v, want [%q]", pathDirs, scriptsDir)
	}
}

// TestExpandHome verifies tilde expansion.
func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}
	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestDiscoverSkills verifies that DiscoverSkills returns deduplicated results
// with first-match-wins semantics.
func TestDiscoverSkills(t *testing.T) {
	cwd := t.TempDir()
	searchDir := t.TempDir()

	writeSkillFile(t, cwd, "go", "Go only.")

	// Put a duplicate "go" and a unique "remote" in searchDir
	for _, name := range []string{"go", "remote"} {
		skillDir := filepath.Join(searchDir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	t.Setenv("AGENT_MUX_SKILL_PATH", searchDir)

	results := DiscoverSkills(cwd)
	if len(results) < 2 {
		t.Fatalf("got %d results, want at least 2: %+v", len(results), results)
	}

	nameMap := make(map[string]string) // name -> source
	for _, r := range results {
		nameMap[r.Name] = r.Source
	}

	if _, ok := nameMap["go"]; !ok {
		t.Fatal("missing 'go' skill in results")
	}
	if _, ok := nameMap["remote"]; !ok {
		t.Fatal("missing 'remote' skill in results")
	}
}

func writeSkillFile(t *testing.T, cwd, name, content string) {
	t.Helper()

	path := filepath.Join(cwd, ".claude", "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
