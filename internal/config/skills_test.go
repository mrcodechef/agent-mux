package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkillsSingleSkill(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Use Go conventions.")

	prompt, pathDirs, err := LoadSkills([]string{"go"}, cwd, "", nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	want := "<skill name=\"go\">\nUse Go conventions.\n</skill>\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsTrimsTrailingNewlineBeforeClosingTag(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Use Go conventions.\n")

	prompt, _, err := LoadSkills([]string{"go"}, cwd, "", nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	want := "<skill name=\"go\">\nUse Go conventions.\n</skill>\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestLoadSkillsMultipleSkillsInOrder(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Go only.")
	writeSkillFile(t, cwd, "review", "Review for regressions.")

	prompt, pathDirs, err := LoadSkills([]string{"go", "review"}, cwd, "", nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	want := "<skill name=\"go\">\nGo only.\n</skill>\n\n<skill name=\"review\">\nReview for regressions.\n</skill>\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
	if len(pathDirs) != 0 {
		t.Fatalf("pathDirs = %#v, want empty", pathDirs)
	}
}

func TestLoadSkillsDeduplicatesNames(t *testing.T) {
	cwd := t.TempDir()
	writeSkillFile(t, cwd, "go", "Only once.")

	prompt, pathDirs, err := LoadSkills([]string{"go", "go"}, cwd, "", nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if strings.Count(prompt, `<skill name="go">`) != 1 {
		t.Fatalf("prompt = %q, want single wrapped skill", prompt)
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

	_, pathDirs, err := LoadSkills([]string{"go"}, cwd, "", nil, "")
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

	_, pathDirs, err := LoadSkills([]string{"go"}, cwd, "", nil, "")
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

	_, _, err := LoadSkills([]string{"missing"}, cwd, "", nil, "")
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

func TestLoadSkillsNotFoundIncludesRoleName(t *testing.T) {
	cwd := t.TempDir()

	_, _, err := LoadSkills([]string{"missing"}, cwd, "", nil, "lifter")
	if err == nil {
		t.Fatal("LoadSkills error = nil, want error")
	}

	msg := err.Error()
	if !strings.Contains(msg, `injected by role "lifter"`) {
		t.Fatalf("error = %q, want role name in error", msg)
	}
}

func TestLoadSkillsEmptyNames(t *testing.T) {
	cwd := t.TempDir()

	prompt, pathDirs, err := LoadSkills(nil, cwd, "", nil, "")
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

	_, _, err := LoadSkills([]string{"../bad"}, cwd, "", nil, "")
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

	_, pathDirs, err := LoadSkills([]string{"go", "review"}, cwd, "", nil, "")
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
func TestLoadSkillsConfigDirFallback(t *testing.T) {
	configDir := t.TempDir()
	cwd := t.TempDir() // deliberately different from configDir

	// Skill lives under configDir, NOT under cwd.
	writeSkillFile(t, configDir, "gaal", "Gaal skill content.")

	prompt, _, err := LoadSkills([]string{"gaal"}, cwd, configDir, nil, "")
	if err != nil {
		t.Fatalf("LoadSkills with configDir fallback: %v", err)
	}

	want := "<skill name=\"gaal\">\nGaal skill content.\n</skill>\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

// TestLoadSkillsConfigDirFallbackWithScriptsDir verifies that scripts/ from the
// fallback (configDir-relative) skill are returned in pathDirs.
func TestLoadSkillsConfigDirFallbackWithScriptsDir(t *testing.T) {
	configDir := t.TempDir()
	cwd := t.TempDir()

	writeSkillFile(t, configDir, "gaal", "Gaal skill content.")
	scriptsDir := filepath.Join(configDir, ".claude", "skills", "gaal", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scripts: %v", err)
	}

	_, pathDirs, err := LoadSkills([]string{"gaal"}, cwd, configDir, nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if len(pathDirs) != 1 || pathDirs[0] != scriptsDir {
		t.Fatalf("pathDirs = %#v, want [%q]", pathDirs, scriptsDir)
	}
}

// TestLoadSkillsCwdTakesPrecedenceOverConfigDir verifies that a skill found
// in cwd is used even when configDir also has the same skill name.
func TestLoadSkillsCwdTakesPrecedenceOverConfigDir(t *testing.T) {
	configDir := t.TempDir()
	cwd := t.TempDir()

	writeSkillFile(t, cwd, "shared", "cwd version")
	writeSkillFile(t, configDir, "shared", "configDir version")

	prompt, _, err := LoadSkills([]string{"shared"}, cwd, configDir, nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if prompt != "<skill name=\"shared\">\ncwd version\n</skill>\n" {
		t.Fatalf("prompt = %q, want cwd version to win", prompt)
	}
}

// TestLoadSkillsConfigDirSameAsCwdNoDoubleSearch verifies that when
// configDir == cwd, passing the same dir as fallback doesn't cause duplicate
// work or errors.
func TestLoadSkillsConfigDirSameAsCwd(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "go", "Go conventions.")

	prompt, _, err := LoadSkills([]string{"go"}, dir, dir, nil, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if prompt != "<skill name=\"go\">\nGo conventions.\n</skill>\n" {
		t.Fatalf("prompt = %q", prompt)
	}
}

// TestLoadSkillsSearchPathFallback verifies that a skill not found in cwd or
// configDir is resolved from search_paths.
func TestLoadSkillsSearchPathFallback(t *testing.T) {
	cwd := t.TempDir()
	configDir := t.TempDir()
	searchDir := t.TempDir()

	// Skill lives under searchDir directly (not inside .claude/skills).
	skillDir := filepath.Join(searchDir, "remote-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Remote skill content."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prompt, _, err := LoadSkills([]string{"remote-skill"}, cwd, configDir, []string{searchDir}, "")
	if err != nil {
		t.Fatalf("LoadSkills with search_path: %v", err)
	}

	want := "<skill name=\"remote-skill\">\nRemote skill content.\n</skill>\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

// TestLoadSkillsCwdWinsOverSearchPath verifies that cwd takes precedence over
// search_paths when the same skill name exists in both.
func TestLoadSkillsCwdWinsOverSearchPath(t *testing.T) {
	cwd := t.TempDir()
	searchDir := t.TempDir()

	writeSkillFile(t, cwd, "shared", "cwd version")

	skillDir := filepath.Join(searchDir, "shared")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("searchPath version"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prompt, _, err := LoadSkills([]string{"shared"}, cwd, "", []string{searchDir}, "")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	if prompt != "<skill name=\"shared\">\ncwd version\n</skill>\n" {
		t.Fatalf("prompt = %q, want cwd version to win over search_path", prompt)
	}
}

// TestLoadSkillsSearchPathWithScriptsDir verifies scripts/ dirs are picked up
// from search_path-resolved skills.
func TestLoadSkillsSearchPathWithScriptsDir(t *testing.T) {
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

	_, pathDirs, err := LoadSkills([]string{"remote-skill"}, cwd, "", []string{searchDir}, "")
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

	results := DiscoverSkills(cwd, "", []string{searchDir})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}

	// "go" should come from cwd (first match wins)
	goResult := results[0]
	if goResult.Name != "go" {
		t.Fatalf("results[0].Name = %q, want %q", goResult.Name, "go")
	}
	if !strings.Contains(goResult.Source, "cwd") {
		t.Fatalf("results[0].Source = %q, want cwd source", goResult.Source)
	}

	// "remote" should come from searchDir
	remoteResult := results[1]
	if remoteResult.Name != "remote" {
		t.Fatalf("results[1].Name = %q, want %q", remoteResult.Name, "remote")
	}
	if !strings.Contains(remoteResult.Source, "search_path") {
		t.Fatalf("results[1].Source = %q, want search_path source", remoteResult.Source)
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
