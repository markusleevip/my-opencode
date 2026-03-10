package skills

import (
	"fmt"
	"myopencode/internal/logging"
	"strings"
)

// Manager provides the public API for the skills system
type Manager struct {
	index      *SkillIndex
	engine     *TriggerEngine
	workingDir string
}

// NewManager creates a new skills manager and automatically loads skills
func NewManager(workingDir ...string) (*Manager, error) {
	manager := &Manager{
		index:  NewSkillIndex(),
		engine: &TriggerEngine{},
	}

	if len(workingDir) > 0 {
		manager.workingDir = workingDir[0]
	}

	// Auto-load skills (non-blocking, errors are logged but don't fail initialization)
	if err := manager.AutoLoad(); err != nil {
		logging.Warn("Skills auto-load encountered issues", "error", err)
	}

	return manager, nil
}

// AutoLoad automatically discovers and loads all skills
func (m *Manager) AutoLoad() error {
	// Discover skills from standard paths
	var skillPaths []string
	var err error

	if m.workingDir != "" {
		skillPaths, err = DiscoverSkills(m.workingDir)
	} else {
		skillPaths, err = DiscoverSkills()
	}
	if err != nil {
		return err
	}

	if len(skillPaths) == 0 {
		logging.Info("No skills directory found - skills feature will be unavailable")
		return nil
	}

	// Load each discovered skill
	loaded := 0
	failed := 0

	for _, path := range skillPaths {
		skill, err := LoadSkill(path)
		if err != nil {
			logging.Warn("Failed to load skill", "path", path, "error", err)
			failed++
			continue
		}

		m.index.Add(skill)
		loaded++
		logging.Debug("Loaded skill", "name", skill.Metadata.Name, "path", path)
	}

	logging.Info("Skills loaded", "count", loaded, "failed", failed)
	return nil
}

// MatchSkills finds skills relevant to the user's input
// Returns an empty slice if no skills match (does not return nil)
func (m *Manager) MatchSkills(input string) []MatchResult {
	if m == nil || m.index == nil {
		return nil
	}

	results := m.engine.MatchSkills(input, m.index)

	if len(results) > 0 {
		logging.Debug("Matched skills", "count", len(results), "skills", results)
	}

	return results
}

// GetSkillContext builds the context string for matched skills
// Returns an empty string if there are no matches
func (m *Manager) GetSkillContext(matches []MatchResult) string {
	if m == nil || m.index == nil || len(matches) == 0 {
		return ""
	}

	return BuildSkillContext(matches, m.index)
}

// ListSkills returns all loaded skills
func (m *Manager) ListSkills() []*Skill {
	if m == nil || m.index == nil {
		return nil
	}
	return m.index.List()
}

// SearchSkills searches for skills by query
func (m *Manager) SearchSkills(query string) []*Skill {
	if m == nil || m.index == nil {
		return nil
	}
	return m.index.Search(query)
}

// Count returns the number of loaded skills
func (m *Manager) Count() int {
	if m == nil || m.index == nil {
		return 0
	}
	return m.index.Count()
}

// Reload clears and reloads all skills
func (m *Manager) Reload() error {
	if m == nil {
		return nil
	}

	// Clear existing index
	m.index = NewSkillIndex()

	// Reload
	return m.AutoLoad()
}

// GetSkill returns a specific skill by name
func (m *Manager) GetSkill(name string) (*Skill, bool) {
	if m == nil || m.index == nil {
		return nil, false
	}
	return m.index.Get(name)
}

// BuildSkillsSummary generates a summary of all loaded skills for the system prompt.
// This tells the LLM what skills are available so it can inform the user.
func (m *Manager) BuildSkillsSummary() string {
	if m == nil || m.index == nil {
		return ""
	}

	skillsList := m.index.List()
	if len(skillsList) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n<skills>\n")
	sb.WriteString("You can use specialized 'skills' to help you with complex tasks. Each skill has a name and a description listed below.\n\n")
	sb.WriteString("Skills are folders of instructions, scripts, and resources that extend your capabilities for specialized tasks. Each skill folder contains:\n")
	sb.WriteString("- **SKILL.md** (required): The main instruction file with YAML frontmatter (name, description) and detailed markdown instructions\n\n")
	sb.WriteString("If a skill seems relevant to the user's current task, you should follow the skill's guidance when responding.\n\n")
	sb.WriteString("Available skills:\n")

	for _, skill := range skillsList {
		desc := skill.Metadata.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", skill.Metadata.Name, skill.Path, desc))
	}

	sb.WriteString("</skills>\n")
	return sb.String()
}
