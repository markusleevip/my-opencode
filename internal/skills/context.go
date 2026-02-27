package skills

import (
	"fmt"
	"strings"
)

// MaxSkillContextLength is the maximum length of skill context to inject
// This helps prevent token limit issues
const MaxSkillContextLength = 8000

// BuildSkillContext generates the context string for matched skills
// This context will be injected into the system message
func BuildSkillContext(matchedSkills []MatchResult, index *SkillIndex) string {
	if len(matchedSkills) == 0 || index == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Active Skills\n\n")
	sb.WriteString("The following skills are available to help with this task. ")
	sb.WriteString("Use these skills' guidance when responding to the user's request.\n\n")

	for i, match := range matchedSkills {
		skill, ok := index.Get(match.SkillName)
		if !ok {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n", skill.Metadata.Name))
		sb.WriteString(fmt.Sprintf("**Description**: %s\n", skill.Metadata.Description))
		// Add the absolute path to the skill directory, so the LLM knows where to find the scripts.
		// Windows paths often contain \ which need to be printed correctly.
		sb.WriteString(fmt.Sprintf("**Skill Directory Path**: `%s`\n", skill.Path))
		sb.WriteString("> **IMPORTANT**: When running scripts from this skill (e.g. `python scripts/...`), you MUST use the **absolute path** by prepending the Skill Directory Path. DO NOT assume the scripts are in your current working directory.\n\n")

		// Include skill body, truncated if necessary
		// Use runes for safe UTF-8 truncation
		runes := []rune(skill.Body)
		maxRunes := MaxSkillContextLength / len(matchedSkills)
		if len(runes) > maxRunes {
			// Truncate with a note
			remaining := maxRunes - 50
			if remaining > 0 {
				sb.WriteString(string(runes[:remaining]))
				sb.WriteString("\n\n[... truncated ...]\n")
			}
		} else {
			sb.WriteString(skill.Body)
		}

		// Add separator between skills (but not after the last one)
		if i < len(matchedSkills)-1 {
			sb.WriteString("\n\n---\n\n")
		}
	}

	return sb.String()
}

// InjectToMessages adds skill context to a system message
// If there's already a system message, it appends to it
// Otherwise, it creates a new system message
func InjectToMessages(messages []Message, skillContext string) []Message {
	if skillContext == "" {
		return messages
	}

	// Find existing system message
	systemIndex := -1
	for i, msg := range messages {
		if msg.Role == "system" {
			systemIndex = i
			break
		}
	}

	if systemIndex >= 0 {
		// Append to existing system message
		messages[systemIndex].Content += skillContext
	} else {
		// Prepend new system message with skill context
		skillMsg := Message{
			Role:    "system",
			Content: skillContext,
		}
		messages = append([]Message{skillMsg}, messages...)
	}

	return messages
}

// Message represents a chat message (simplified version for skills package)
// This is a minimal definition to avoid import cycles
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
