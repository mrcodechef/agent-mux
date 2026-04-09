package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/buildoak/agent-mux/internal/sanitize"
	"gopkg.in/yaml.v3"
)

// SkillSearchResult describes a discovered skill and where it was found.
type SkillSearchResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// LoadSkills loads skill SKILL.md files by name and returns a concatenated prompt
// block and a list of scripts directories to add to PATH.
//
// Resolution order (convention-based):
//  1. AGENT_MUX_SKILL_PATH entries (colon-separated, prepended)
//  2. <cwd>/.agent-mux/skills/<name>/SKILL.md
//  3. <cwd>/.claude/skills/<name>/SKILL.md
//  4. ~/.agent-mux/skills/<name>/SKILL.md
//  5. ~/.claude/skills/<name>/SKILL.md
//
// sourceName is used only for error messages — it names the profile that requested the skill.
func LoadSkills(names []string, cwd string, sourceName string) (prompt string, pathDirs []string, err error) {
	if len(names) == 0 {
		return "", nil, nil
	}

	// Build the ordered list of search roots.
	roots := buildSearchRoots(cwd)

	seen := make(map[string]struct{}, len(names))
	refs := make([]string, 0, len(names))
	pathDirs = make([]string, 0)

	for _, name := range names {
		name = strings.TrimSpace(name)
		if err := sanitize.ValidateBasename(name); err != nil {
			return "", nil, fmt.Errorf("invalid skill name %q: %w", name, err)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		resolvedRoot, content, readErr := resolveSkill(name, roots)
		if readErr != nil {
			return "", nil, skillNotFoundError(name, sourceName, roots)
		}

		skillPath := filepath.Join(resolvedRoot, name, "SKILL.md")
		desc := extractSkillDescription(content)
		refs = append(refs, fmt.Sprintf("- %s: %s → %s", name, desc, skillPath))

		scriptsDir := filepath.Join(resolvedRoot, name, "scripts")
		info, statErr := os.Stat(scriptsDir)
		if statErr == nil && info.IsDir() {
			pathDirs = append(pathDirs, scriptsDir)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return "", nil, fmt.Errorf("stat scripts dir for skill %q: %w", name, statErr)
		}
	}

	if len(refs) == 0 {
		return "", pathDirs, nil
	}
	block := "Available skills (read the SKILL.md file when you need the skill's instructions):\n" + strings.Join(refs, "\n") + "\n"
	return block, pathDirs, nil
}

// searchRoot describes one root directory to search for skills and a
// human-readable label for error messages.
type searchRoot struct {
	dir   string
	label string
}

// buildSearchRoots returns the ordered list of skill search roots using
// directory conventions and AGENT_MUX_SKILL_PATH env var.
func buildSearchRoots(cwd string) []searchRoot {
	roots := make([]searchRoot, 0, 8)

	// 0. AGENT_MUX_SKILL_PATH entries (prepended, colon-separated)
	if envPath := os.Getenv("AGENT_MUX_SKILL_PATH"); envPath != "" {
		for _, sp := range strings.Split(envPath, ":") {
			sp = strings.TrimSpace(sp)
			if sp == "" {
				continue
			}
			expanded := expandHome(sp)
			roots = append(roots, searchRoot{
				dir:   expanded,
				label: fmt.Sprintf("env (%s)", sp),
			})
		}
	}

	// 1. <cwd>/.agent-mux/skills
	roots = append(roots, searchRoot{
		dir:   filepath.Join(cwd, ".agent-mux", "skills"),
		label: "cwd (.agent-mux/skills)",
	})

	// 2. <cwd>/.claude/skills
	roots = append(roots, searchRoot{
		dir:   filepath.Join(cwd, ".claude", "skills"),
		label: "cwd (.claude/skills)",
	})

	// 3. ~/.agent-mux/skills
	if homeDir, err := os.UserHomeDir(); err == nil {
		roots = append(roots, searchRoot{
			dir:   filepath.Join(homeDir, ".agent-mux", "skills"),
			label: "global (~/.agent-mux/skills)",
		})
		// 4. ~/.claude/skills
		roots = append(roots, searchRoot{
			dir:   filepath.Join(homeDir, ".claude", "skills"),
			label: "global (~/.claude/skills)",
		})
	}

	return roots
}

// resolveSkill tries each root in order and returns the root directory,
// file content, and nil error on success. On failure it returns a non-nil error.
func resolveSkill(name string, roots []searchRoot) (resolvedRoot string, content []byte, err error) {
	for _, root := range roots {
		skillFile := filepath.Join(root.dir, name, "SKILL.md")
		data, readErr := os.ReadFile(skillFile)
		if readErr == nil {
			return root.dir, data, nil
		}
		if !os.IsNotExist(readErr) {
			return "", nil, fmt.Errorf("read skill %q from %s: %w", name, root.label, readErr)
		}
	}
	return "", nil, fmt.Errorf("not found")
}

// skillNotFoundError builds a detailed error message including the source name
// (profile or caller context) and all paths that were searched.
func skillNotFoundError(skillName, sourceName string, roots []searchRoot) error {
	var b strings.Builder
	fmt.Fprintf(&b, "skill %q not found", skillName)
	if sourceName != "" {
		fmt.Fprintf(&b, " (requested by %q)", sourceName)
	}
	b.WriteString(". Searched:\n")
	for _, root := range roots {
		fmt.Fprintf(&b, "  - %s: %s\n", root.label, filepath.Join(root.dir, skillName, "SKILL.md"))
	}
	avail := availableSkillsFromRoots(roots)
	if len(avail) > 0 {
		fmt.Fprintf(&b, "Available skills: %v", avail)
	} else {
		b.WriteString("No skills found in any search path.")
	}
	return fmt.Errorf("%s", b.String())
}

// DiscoverSkills scans all known search roots and returns deduplicated skills
// with the winning (first-match) path and source label. Used by `config skills`.
func DiscoverSkills(cwd string) []SkillSearchResult {
	roots := buildSearchRoots(cwd)
	seen := make(map[string]struct{})
	var results []SkillSearchResult

	for _, root := range roots {
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if _, ok := seen[name]; ok {
				continue
			}
			// Verify SKILL.md exists
			skillFile := filepath.Join(root.dir, name, "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			seen[name] = struct{}{}
			results = append(results, SkillSearchResult{
				Name:   name,
				Path:   skillFile,
				Source: root.label,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// availableSkillsFromRoots returns a deduplicated, sorted list of skill names
// found across all search roots.
func availableSkillsFromRoots(roots []searchRoot) []string {
	seen := make(map[string]struct{})
	for _, root := range roots {
		collectSkills(root.dir, seen)
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func collectSkills(skillsRoot string, seen map[string]struct{}) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Only include directories that actually contain SKILL.md — otherwise ghost
		// directories (e.g. stale checkouts, partial installs) appear in the
		// "Available skills" error message and mislead the user.
		skillFile := filepath.Join(skillsRoot, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}
		seen[entry.Name()] = struct{}{}
	}
}

// extractSkillDescription parses YAML frontmatter from a SKILL.md file and
// returns a short description string. Falls back to the skill name if parsing
// fails or no description is present.
func extractSkillDescription(content []byte) string {
	fm, _, err := splitFrontmatter(content)
	if err != nil || len(fm) == 0 {
		return "(no description)"
	}
	var parsed struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(fm, &parsed); err != nil || parsed.Description == "" {
		return "(no description)"
	}
	desc := strings.TrimSpace(parsed.Description)
	firstLine := strings.SplitN(desc, "\n", 2)[0]
	if len(firstLine) > 100 {
		firstLine = firstLine[:97] + "..."
	}
	return firstLine
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
