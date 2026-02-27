// Package skills provides automatic skill discovery, loading, and triggering.
// It scans the user's skills directory (~/.my-opencode/skills/) and automatically
// injects relevant skill context based on user input.
package skills

import (
	"strings"
	"sync"
	"time"
)

// SkillMetadata defines the frontmatter structure from SKILL.md
type SkillMetadata struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	License     string   `json:"license,omitempty" yaml:"license"`
	Version     string   `json:"version,omitempty" yaml:"version"`
	Tags        []string `json:"tags,omitempty" yaml:"tags"`
}

// Skill represents a loaded skill
type Skill struct {
	Metadata   SkillMetadata
	Path       string
	NameLower  string   // Cached lowercase name
	DescLower  string   // Cached lowercase description
	Body       string   // Full markdown body (after frontmatter)
	Scripts    []string // Available scripts in scripts/
	References []string // Reference documents in references/
	Assets     []string // Asset files in assets/
	LoadedAt   time.Time
}

// SkillIndex is the in-memory index of all loaded skills
type SkillIndex struct {
	mu     sync.RWMutex
	skills map[string]*Skill   // name -> Skill
	byTag  map[string][]string // tag -> []name
}

// NewSkillIndex creates a new skill index
func NewSkillIndex() *SkillIndex {
	return &SkillIndex{
		skills: make(map[string]*Skill),
		byTag:  make(map[string][]string),
	}
}

// Add adds a skill to the index
func (idx *SkillIndex) Add(skill *Skill) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.skills[skill.Metadata.Name] = skill

	// Index by tags
	for _, tag := range skill.Metadata.Tags {
		idx.byTag[tag] = append(idx.byTag[tag], skill.Metadata.Name)
	}
}

// Get retrieves a skill by name
func (idx *SkillIndex) Get(name string) (*Skill, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	skill, ok := idx.skills[name]
	return skill, ok
}

// List returns all enabled skills
func (idx *SkillIndex) List() []*Skill {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]*Skill, 0, len(idx.skills))
	for _, skill := range idx.skills {
		result = append(result, skill)
	}

	return result
}

// Count returns the number of loaded skills
func (idx *SkillIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.skills)
}

// Search finds skills matching a query in name, description, or tags
func (idx *SkillIndex) Search(query string) []*Skill {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	query = normalizeText(query)
	var results []*Skill

	for name, skill := range idx.skills {
		if matchesQuery(query, skill, name) {
			results = append(results, skill)
		}
	}

	return results
}

// MatchResult represents a skill match result
type MatchResult struct {
	SkillName string
	Score     float64
	Reason    string
}

// normalizeText converts text to lowercase and trims whitespace
func normalizeText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

// matchesQuery checks if a skill matches the search query
func matchesQuery(query string, skill *Skill, name string) bool {
	if strings.Contains(normalizeText(name), query) {
		return true
	}
	if strings.Contains(normalizeText(skill.Metadata.Description), query) {
		return true
	}
	for _, tag := range skill.Metadata.Tags {
		if strings.Contains(normalizeText(tag), query) {
			return true
		}
	}
	return false
}
