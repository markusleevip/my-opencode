package skills

import (
	"sort"
	"strings"
	"unicode"
)

// MatchThreshold is the minimum score for a skill to be considered relevant
const MatchThreshold = 0.3

// TriggerEngine handles skill triggering logic based on user input
type TriggerEngine struct{}

// MatchSkills finds skills relevant to the user's input
// Returns a list of matched skills sorted by relevance score
func (e *TriggerEngine) MatchSkills(input string, index *SkillIndex) []MatchResult {
	if index == nil {
		return nil
	}

	// Extract keywords from input
	keywords := extractKeywords(input)
	if len(keywords) == 0 {
		return nil
	}

	var results []MatchResult

	index.mu.RLock()
	defer index.mu.RUnlock()

	// Calculate score for each skill
	for name, skill := range index.skills {
		// Use cached lowercase variants for performance
		score := calculateSkillScoreWithCache(keywords, skill)

		if score >= MatchThreshold {
			results = append(results, MatchResult{
				SkillName: name,
				Score:     score,
				Reason:    generateMatchReason(keywords, skill),
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to Top-3 (Top-K) to prevent context bloat
	if len(results) > 3 {
		results = results[:3]
	}

	return results
}

// extractKeywords extracts meaningful keywords from user input
func extractKeywords(input string) []string {
	// Split on non-alphanumeric characters
	words := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Common stopwords to filter out
	stopwords := map[string]bool{
		// English
		"the": true, "a": true, "an": true, "is": true,
		"are": true, "was": true, "were": true, "be": true,
		"have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true,
		"to": true, "of": true, "in": true, "for": true,
		"on": true, "with": true, "at": true, "by": true,
		"from": true, "as": true, "into": true, "through": true,
		"and": true, "or": true, "if": true,
		"that": true, "this": true, "it": true, "me": true,
		"my": true, "your": true, "you": true, "we": true,
		"our": true, "they": true, "their": true, "them": true,
		"what": true, "which": true, "who": true, "how": true,
		"when": true, "where": true, "why": true, "all": true,
		"each": true, "every": true, "both": true, "few": true,
		"more": true, "most": true, "other": true, "some": true,
		"such": true, "no": true, "nor": true,
		"only": true, "own": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "can": true,
		"just": true, "now": true, "i": true, "help": true,
		"please": true, "want": true, "need": true,
	}

	// Filter and normalize keywords
	var keywords []string
	for _, word := range words {
		word = strings.ToLower(word)

		// Skip stopwords and short words
		if len(word) <= 2 || stopwords[word] {
			continue
		}

		keywords = append(keywords, word)
	}

	return keywords
}

// calculateSkillScoreWithCache computes a relevance score using cached lowercase fields
func calculateSkillScoreWithCache(keywords []string, skill *Skill) float64 {
	if len(keywords) == 0 {
		return 0.0
	}

	score := 0.0
	descLower := skill.DescLower
	nameLower := skill.NameLower

	for _, kw := range keywords {
		// Name match (highest weight - 3.0)
		if strings.Contains(nameLower, kw) {
			score += 3.0
		}

		// Description match (medium weight - 1.0)
		if strings.Contains(descLower, kw) {
			score += 1.0
		}

		// Tag match (high weight - 2.0)
		for _, tag := range skill.Metadata.Tags {
			if strings.Contains(strings.ToLower(tag), kw) {
				score += 2.0
			}
		}

		// Partial match for longer keywords (fuzzy matching)
		if len(kw) >= 4 {
			// Check for substring matches
			for i := 3; i < len(kw); i++ {
				prefix := kw[:i]
				if strings.Contains(descLower, prefix) {
					score += 0.3 // Small bonus for partial matches
					break
				}
			}
		}
	}

	// Normalize by number of keywords to prevent bias
	normalizedScore := score / float64(len(keywords))

	// Boost score if multiple keywords match
	matchedKeywords := 0
	for _, kw := range keywords {
		if strings.Contains(nameLower, kw) || strings.Contains(descLower, kw) {
			matchedKeywords++
		}
	}

	if matchedKeywords >= 2 {
		normalizedScore *= 1.2 // 20% boost for multiple matches
	}
	if matchedKeywords >= 3 {
		normalizedScore *= 1.3 // 30% boost for three or more matches
	}

	return normalizedScore
}

// generateMatchReason creates a human-readable explanation for why a skill matched
func generateMatchReason(keywords []string, skill *Skill) string {
	descLower := strings.ToLower(skill.Metadata.Description)
	nameLower := strings.ToLower(skill.Metadata.Name)

	var matchedParts []string

	// Check name match
	for _, kw := range keywords {
		if strings.Contains(nameLower, kw) {
			matchedParts = append(matchedParts, "name")
			break
		}
	}

	// Check description match
	matchedInDesc := []string{}
	for _, kw := range keywords {
		if strings.Contains(descLower, kw) {
			matchedInDesc = append(matchedInDesc, kw)
		}
	}
	if len(matchedInDesc) > 0 {
		matchedParts = append(matchedParts, "description")
	}

	// Check tag match
	matchedInTags := []string{}
	for _, kw := range keywords {
		for _, tag := range skill.Metadata.Tags {
			if strings.Contains(strings.ToLower(tag), kw) {
				matchedInTags = append(matchedInTags, kw)
				break
			}
		}
	}
	if len(matchedInTags) > 0 {
		matchedParts = append(matchedParts, "tags")
	}

	if len(matchedParts) == 0 {
		return "weak keyword match"
	}

	return "matched in: " + strings.Join(matchedParts, ", ")
}
