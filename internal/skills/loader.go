package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSkill loads a single skill from its directory
func LoadSkill(skillPath string) (*Skill, error) {
	// Find SKILL.md (case-insensitive)
	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		// Try lowercase
		skillMdPath = filepath.Join(skillPath, "skill.md")
		if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SKILL.md not found in %s", skillPath)
		}
	}

	// Read the file
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	// Parse frontmatter and body
	metadata, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Discover optional subdirectories
	scripts := discoverSubdirectory(filepath.Join(skillPath, "scripts"))
	references := discoverSubdirectory(filepath.Join(skillPath, "references"))
	assets := discoverSubdirectory(filepath.Join(skillPath, "assets"))

	return &Skill{
		Metadata:   metadata,
		Path:       skillPath,
		NameLower:  strings.ToLower(metadata.Name),
		DescLower:  strings.ToLower(metadata.Description),
		Body:       body,
		Scripts:    scripts,
		References: references,
		Assets:     assets,
	}, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content
// Frontmatter format:
// ---
// name: skill-name
// description: Skill description
// ---
// Body content here...
func parseFrontmatter(content []byte) (SkillMetadata, string, error) {
	contentStr := string(content)

	// Match frontmatter between --- delimiters
	// Pattern: ^---\r?\n(.*?)\r?\n---\r?\n(.*)$
	re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n(.*)$`)
	matches := re.FindStringSubmatch(contentStr)

	if len(matches) < 3 {
		// No frontmatter found, return empty metadata with full content as body
		return SkillMetadata{}, contentStr, nil
	}

	// Parse YAML frontmatter
	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(matches[1]), &metadata); err != nil {
		return SkillMetadata{}, "", err
	}

	// Body is everything after the frontmatter
	body := strings.TrimSpace(matches[2])

	return metadata, body, nil
}

// discoverSubdirectory lists files in a subdirectory if it exists
func discoverSubdirectory(dirPath string) []string {
	var files []string

	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return files
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files
}
