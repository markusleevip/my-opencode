package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// defaultSkillPaths defines the standard paths to search for skills
var defaultSkillPaths = []string{
	"~/.my-opencode/skills", // Cross-platform: ~/.my-opencode/skills
}

// DiscoverSkills automatically discovers skills in the user's skills directory
// Returns a list of skill directory paths that contain SKILL.md
func DiscoverSkills() ([]string, error) {
	var skills []string

	for _, pathPattern := range defaultSkillPaths {
		// Expand path (handle ~ and environment variables)
		path := expandPath(pathPattern)

		// Check if directory exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Directory doesn't exist, silently skip
			// This is normal for first-time users
			continue
		}

		// Read directory entries
		entries, err := os.ReadDir(path)
		if err != nil {
			// Can't read directory, skip
			continue
		}

		// Find valid skills (directories containing SKILL.md)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(path, entry.Name())
			skillMdPath := filepath.Join(skillPath, "SKILL.md")

			// Check for case-insensitive SKILL.md
			if _, err := os.Stat(skillMdPath); err == nil {
				skills = append(skills, skillPath)
			} else {
				// Try lowercase skill.md for compatibility
				skillMdPathLower := filepath.Join(skillPath, "skill.md")
				if _, err := os.Stat(skillMdPathLower); err == nil {
					skills = append(skills, skillPath)
				}
			}
		}
	}

	return skills, nil
}

// expandPath expands path variables like ~ and environment variables
func expandPath(path string) string {
	// Handle ~ (home directory)
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to environment variables
			if runtime.GOOS == "windows" {
				home = os.Getenv("USERPROFILE")
			} else {
				home = os.Getenv("HOME")
			}
		}
		if home != "" {
			return filepath.Join(home, strings.TrimPrefix(path[1:], "/"))
		}
	}

	// Expand environment variables
	return os.ExpandEnv(path)
}
