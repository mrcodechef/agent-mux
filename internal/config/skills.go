package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/buildoak/agent-mux/internal/sanitize"
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
// Resolution order:
//  1. <cwd>/.claude/skills/<name>/SKILL.md
//  2. <configDir>/.claude/skills/<name>/SKILL.md  (if configDir != "" and != cwd)
//  3. Each path in searchPaths: <search_path>/<name>/SKILL.md
//
// roleName is used only for error messages — it names the role that requested the skill.
func LoadSkills(names []string, cwd string, configDir string, searchPaths []string, roleName string) (prompt string, pathDirs []string, err error) {
	if len(names) == 0 {
		return "", nil, nil
	}

	// Build the ordered list of search roots.
	roots := buildSearchRoots(cwd, configDir, searchPaths)

	seen := make(map[string]struct{}, len(names))
	blocks := make([]string, 0, len(names))
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
			return "", nil, skillNotFoundError(name, roleName, roots)
		}

		trimmed := strings.TrimRight(string(content), "\r\n")
		block := fmt.Sprintf("<skill name=%q>\n%s\n</skill>\n", name, trimmed)
		blocks = append(blocks, block)

		scriptsDir := filepath.Join(resolvedRoot, name, "scripts")
		info, statErr := os.Stat(scriptsDir)
		if statErr == nil && info.IsDir() {
			pathDirs = append(pathDirs, scriptsDir)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return "", nil, fmt.Errorf("stat scripts dir for skill %q: %w", name, statErr)
		}
	}

	return strings.Join(blocks, "\n"), pathDirs, nil
}

// searchRoot describes one root directory to search for skills and a
// human-readable label for error messages.
type searchRoot struct {
	dir   string
	label string
}

// buildSearchRoots returns the ordered list of skill search roots.
func buildSearchRoots(cwd string, configDir string, searchPaths []string) []searchRoot {
	roots := make([]searchRoot, 0, 2+len(searchPaths))

	// 1. CWD-relative
	roots = append(roots, searchRoot{
		dir:   filepath.Join(cwd, ".claude", "skills"),
		label: "cwd (.claude/skills)",
	})

	// 2. ConfigDir-relative (only if different from cwd)
	if configDir != "" && configDir != cwd {
		roots = append(roots, searchRoot{
			dir:   filepath.Join(configDir, ".claude", "skills"),
			label: "configDir (.claude/skills)",
		})
	}

	// 3. Explicit search_paths
	for _, sp := range searchPaths {
		expanded := expandHome(sp)
		roots = append(roots, searchRoot{
			dir:   expanded,
			label: fmt.Sprintf("search_path (%s)", sp),
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

// skillNotFoundError builds a detailed error message including the role name
// and all paths that were searched.
func skillNotFoundError(skillName, roleName string, roots []searchRoot) error {
	var b strings.Builder
	fmt.Fprintf(&b, "skill %q not found", skillName)
	if roleName != "" {
		fmt.Fprintf(&b, " (injected by role %q)", roleName)
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
func DiscoverSkills(cwd string, configDir string, searchPaths []string) []SkillSearchResult {
	roots := buildSearchRoots(cwd, configDir, searchPaths)
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

func availableSkills(skillsRoot string) []string {
	return availableSkillsFromRoots([]searchRoot{{dir: skillsRoot}})
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
