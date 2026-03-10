package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkills(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// Test 1: No skills directory exists
	originalPath := defaultSkillPaths[0]
	defaultSkillPaths[0] = filepath.Join(tmpDir, "nonexistent")

	skills, err := DiscoverSkills()
	if err != nil {
		t.Errorf("DiscoverSkills should not error for nonexistent directory: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("DiscoverSkills should return empty slice for nonexistent directory")
	}

	// Test 2: Skills directory exists but no skills
	os.MkdirAll(skillsDir, 0755)
	defaultSkillPaths[0] = skillsDir

	skills, err = DiscoverSkills()
	if err != nil {
		t.Errorf("DiscoverSkills should not error for empty directory: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("DiscoverSkills should return empty slice for empty directory")
	}

	// Test 3: Valid skill directory
	validSkillDir := filepath.Join(skillsDir, "test-skill")
	os.MkdirAll(validSkillDir, 0755)
	skillMdPath := filepath.Join(validSkillDir, "SKILL.md")
	os.WriteFile(skillMdPath, []byte(`---
name: test-skill
description: A test skill for testing
---
Test skill body content`), 0644)

	skills, err = DiscoverSkills()
	if err != nil {
		t.Errorf("DiscoverSkills should not error for valid skill: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("DiscoverSkills should return 1 skill, got %d", len(skills))
	}

	// Test 4: Project-level skill directory
	projectDir := filepath.Join(tmpDir, "project")
	projectSkillsDir := filepath.Join(projectDir, ".opencode", "skills")
	validProjectSkillDir := filepath.Join(projectSkillsDir, "project-test-skill")
	os.MkdirAll(validProjectSkillDir, 0755)
	projectSkillMdPath := filepath.Join(validProjectSkillDir, "SKILL.md")
	os.WriteFile(projectSkillMdPath, []byte(`---
name: project-test-skill
description: A project level test skill for testing
---
Test project skill body content`), 0644)

	// Since we specify projectDir, we expect 2 skills (1 global + 1 project)
	skills, err = DiscoverSkills(projectDir)
	if err != nil {
		t.Errorf("DiscoverSkills should not error for valid project skill: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("DiscoverSkills with project dir should return 2 skills, got %d", len(skills))
	}

	// Restore original path
	defaultSkillPaths[0] = originalPath
}

func TestLoadSkill(t *testing.T) {
	// Create a temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill for testing
license: MIT
tags:
  - test
  - demo
---
This is the test skill body content.
It has multiple lines.
`
	os.WriteFile(skillMdPath, []byte(skillContent), 0644)

	// Test loading the skill
	skill, err := LoadSkill(skillDir)
	if err != nil {
		t.Errorf("LoadSkill should not error: %v", err)
	}

	if skill.Metadata.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got '%s'", skill.Metadata.Name)
	}

	if skill.Metadata.Description != "A test skill for testing" {
		t.Errorf("Expected description 'A test skill for testing', got '%s'", skill.Metadata.Description)
	}

	if skill.Metadata.License != "MIT" {
		t.Errorf("Expected license 'MIT', got '%s'", skill.Metadata.License)
	}

	if len(skill.Metadata.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(skill.Metadata.Tags))
	}

	if skill.Body != "This is the test skill body content.\nIt has multiple lines." {
		t.Errorf("Unexpected body: %s", skill.Body)
	}
}

func TestCalculateSkillScore(t *testing.T) {
	skill := &Skill{
		Metadata: SkillMetadata{
			Name:        "frontend-design",
			Description: "Create distinctive, production-grade frontend interfaces",
			Tags:        []string{"frontend", "web", "ui"},
		},
		NameLower: "frontend-design",
		DescLower: "create distinctive, production-grade frontend interfaces",
	}

	// Test 1: Strong match
	keywords := []string{"react", "dashboard", "frontend"}
	score := calculateSkillScoreWithCache(keywords, skill)
	if score < 0.3 {
		t.Errorf("Expected score >= 0.3 for strong match, got %f", score)
	}

	// Test 2: Weak match
	keywords = []string{"cooking", "recipe"}
	score = calculateSkillScoreWithCache(keywords, skill)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 for weak match, got %f", score)
	}

	// Test 3: Name match (should be higher)
	keywords = []string{"frontend"}
	score = calculateSkillScoreWithCache(keywords, skill)
	if score < 1.0 {
		t.Errorf("Expected score >= 1.0 for name match, got %f", score)
	}
}

func TestSkillIndex(t *testing.T) {
	idx := NewSkillIndex()

	skill := &Skill{
		Metadata: SkillMetadata{
			Name:        "test-skill",
			Description: "A test skill",
			Tags:        []string{"test", "demo"},
		},
	}

	// Test Add
	idx.Add(skill)
	if idx.Count() != 1 {
		t.Errorf("Expected count 1 after add, got %d", idx.Count())
	}

	// Test Get
	retrieved, ok := idx.Get("test-skill")
	if !ok {
		t.Error("Expected to retrieve test-skill")
	}
	if retrieved.Metadata.Name != "test-skill" {
		t.Error("Retrieved wrong skill")
	}

	// Test List
	skills := idx.List()
	if len(skills) != 1 {
		t.Errorf("Expected 1 skill in list, got %d", len(skills))
	}

	// Test Search
	results := idx.Search("test")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'test' search, got %d", len(results))
	}
}
