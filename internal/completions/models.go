package completions

import (
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

// modelContextGroup implements dialog.CompletionProvider for /model command autocomplete.
type modelContextGroup struct{}

func (m *modelContextGroup) GetId() string {
	return "model"
}

func (m *modelContextGroup) GetEntry() dialog.CompletionItemI {
	return dialog.NewCompletionItem(dialog.CompletionItem{
		Title: "Models",
		Value: "model",
	})
}

func (m *modelContextGroup) GetChildEntries(query string) ([]dialog.CompletionItemI, error) {
	var allModels []string
	displayMap := make(map[string]string) // id → display name

	for id, model := range models.SupportedModels {
		// Build a human-friendly display ID: "provider/apiModel"
		providerStr := string(model.Provider)
		modelDisplay := providerStr + "/" + model.APIModel
		displayMap[string(id)] = modelDisplay
		allModels = append(allModels, string(id))
	}

	// Filter by query (fuzzy search on either internal ID or display name)
	var matches []string
	if query == "" {
		matches = allModels
	} else {
		// Search in both internal IDs and display names
		var searchPool []string
		for _, id := range allModels {
			searchPool = append(searchPool, id)
			searchPool = append(searchPool, displayMap[id])
		}
		rawMatches := fuzzy.Find(query, searchPool)
		seen := make(map[string]bool)
		for _, m := range rawMatches {
			// Normalize: if matched display name, find original id
			clean := m
			if strings.Contains(m, "/") {
				clean = strings.ReplaceAll(m, "/", ".")
			}
			if !seen[clean] {
				seen[clean] = true
				matches = append(matches, clean)
			}
		}
	}

	items := make([]dialog.CompletionItemI, 0, len(matches))
	for _, id := range matches {
		display := displayMap[id]
		if display == "" {
			display = id
		}
		items = append(items, dialog.NewCompletionItem(dialog.CompletionItem{
			Title: display,
			Value: id, // internal ID used for actual switching
		}))
	}
	return items, nil
}

// NewModelContextGroup creates a CompletionProvider for model ID autocomplete.
func NewModelContextGroup() dialog.CompletionProvider {
	return &modelContextGroup{}
}
