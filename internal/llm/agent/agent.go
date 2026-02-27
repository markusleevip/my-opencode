package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"myopencode/internal/config"
	"myopencode/internal/llm/models"
	"myopencode/internal/llm/prompt"
	"myopencode/internal/llm/provider"
	"myopencode/internal/llm/tools"
	"myopencode/internal/logging"
	"myopencode/internal/message"
	"myopencode/internal/permission"
	"myopencode/internal/permission/types"
	"myopencode/internal/pubsub"
	"myopencode/internal/session"
)

// Common errors
var (
	ErrRequestCancelled = errors.New("request cancelled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

type AgentEvent struct {
	Type    AgentEventType
	Message message.Message
	Error   error

	// When summarizing
	SessionID string
	Progress  string
	Done      bool
}

type Service interface {
	pubsub.Suscriber[AgentEvent]
	Model() models.Model
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error)
	Summarize(ctx context.Context, sessionID string) error
	SetPermissionsMode(mode types.PermissionsMode)
	PermissionsMode() types.PermissionsMode
	SetSkillsManager(manager SkillsManager)
}

// SkillsManager defines the interface for skills functionality
// This interface avoids direct dependency on the skills package
type SkillsManager interface {
	MatchSkills(input string) []SkillMatchResult
	GetSkillContext(matches []SkillMatchResult) string
	GetSkillsSummary() string
}

// SkillMatchResult matches the skills.MatchResult structure
type SkillMatchResult struct {
	SkillName string
	Score     float64
	Reason    string
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	sessions      session.Service
	messages      message.Service
	skillsManager SkillsManager

	tools    []tools.BaseTool
	provider provider.Provider

	titleProvider     provider.Provider
	summarizeProvider provider.Provider

	activeRequests  sync.Map
	permissionsMode types.PermissionsMode
}

func NewAgent(
	agentName config.AgentName,
	sessions session.Service,
	messages message.Service,
	agentTools []tools.BaseTool,
) (Service, error) {
	agentProvider, err := createAgentProvider(agentName, types.ActManual)
	if err != nil {
		return nil, err
	}
	var titleProvider provider.Provider
	// Only generate titles for the coder agent
	if agentName == config.AgentCoder {
		titleProvider, err = createAgentProvider(config.AgentTitle, types.ActManual)
		if err != nil {
			return nil, err
		}
	}
	var summarizeProvider provider.Provider
	if agentName == config.AgentCoder {
		summarizeProvider, err = createAgentProvider(config.AgentSummarizer, types.ActManual)
		if err != nil {
			return nil, err
		}
	}

	agent := &agent{
		Broker:            pubsub.NewBroker[AgentEvent](),
		provider:          agentProvider,
		messages:          messages,
		sessions:          sessions,
		tools:             agentTools,
		titleProvider:     titleProvider,
		summarizeProvider: summarizeProvider,
		activeRequests:    sync.Map{},
		permissionsMode:   types.ActManual,
	}

	return agent, nil
}

func (a *agent) SetPermissionsMode(mode types.PermissionsMode) {
	a.permissionsMode = mode
	// Re-create provider to update system prompt
	p, err := createAgentProvider(config.AgentCoder, mode)
	if err == nil {
		a.provider = p
	}
}

func (a *agent) PermissionsMode() types.PermissionsMode {
	return a.permissionsMode
}

func (a *agent) SetSkillsManager(manager SkillsManager) {
	a.skillsManager = manager

	// Rebuild provider with skills summary in system prompt
	// This is necessary because the provider stores a VALUE copy of options,
	// so we must create a new provider with the skills summary baked in.
	if manager != nil {
		summary := manager.GetSkillsSummary()
		if summary != "" {
			p, err := createAgentProvider(config.AgentCoder, a.permissionsMode, summary)
			if err == nil {
				a.provider = p
				logging.Info("Rebuilt provider with skills summary in system prompt")
			} else {
				logging.Warn("Failed to rebuild provider with skills", "error", err)
			}
		}
	}
}

// injectSkillsContext injects skills context into the system message
func injectSkillsContext(messages []message.Message, skillContext string) []message.Message {
	if skillContext == "" {
		return messages
	}

	// Find the last User message to append the context to
	// We use User role instead of System role because some providers (like OpenAI)
	// ignore runtime System messages and only use their configured system prompt.
	userIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.User {
			userIndex = i
			break
		}
	}

	formattedContext := "\n\n[System Note: Relevant Skill Instructions]\n" + skillContext

	if userIndex >= 0 {
		// Append to the last user message
		messages[userIndex].Parts = append(messages[userIndex].Parts, message.TextContent{Text: formattedContext})
	} else {
		// Prepend a new user message with skills context
		skillMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: formattedContext}},
		}
		messages = append([]message.Message{skillMsg}, messages...)
	}

	return messages
}

func (a *agent) Model() models.Model {
	return a.provider.Model()
}

func (a *agent) Cancel(sessionID string) {
	// Cancel regular requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Request cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}

	// Also check for summarize requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID + "-summarize"); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Summarize cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}
}

func (a *agent) IsBusy() bool {
	busy := false
	a.activeRequests.Range(func(key, value interface{}) bool {
		if cancelFunc, ok := value.(context.CancelFunc); ok {
			if cancelFunc != nil {
				busy = true
				return false // Stop iterating
			}
		}
		return true // Continue iterating
	})
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Load(sessionID)
	return busy
}

func (a *agent) generateTitle(ctx context.Context, sessionID string, content string) error {
	if content == "" {
		return nil
	}
	if a.titleProvider == nil {
		return nil
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	parts := []message.ContentPart{message.TextContent{Text: content}}
	response, err := a.titleProvider.SendMessages(
		ctx,
		[]message.Message{
			{
				Role:  message.User,
				Parts: parts,
			},
		},
		make([]tools.BaseTool, 0),
	)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(strings.ReplaceAll(response.Content, "\n", " "))
	if title == "" {
		return nil
	}

	session.Title = title
	_, err = a.sessions.Save(ctx, session)
	return err
}

func (a *agent) err(err error) AgentEvent {
	return AgentEvent{
		Type:  AgentEventTypeError,
		Error: err,
	}
}

func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
	if !a.provider.Model().SupportsAttachments && attachments != nil {
		attachments = nil
	}
	events := make(chan AgentEvent)
	if a.IsSessionBusy(sessionID) {
		return nil, ErrSessionBusy
	}

	genCtx, cancel := context.WithCancel(ctx)

	a.activeRequests.Store(sessionID, cancel)
	go func() {
		logging.Debug("Request started", "sessionID", sessionID)
		defer logging.RecoverPanic("agent.Run", func() {
			events <- a.err(fmt.Errorf("panic while running the agent"))
		})
		var attachmentParts []message.ContentPart
		for _, attachment := range attachments {
			attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
		}
		result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
		if result.Error != nil && !errors.Is(result.Error, ErrRequestCancelled) && !errors.Is(result.Error, context.Canceled) {
			logging.ErrorPersist(result.Error.Error())
		}
		logging.Debug("Request completed", "sessionID", sessionID)
		a.activeRequests.Delete(sessionID)
		cancel()
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) AgentEvent {
	cfg := config.Get()
	// List existing messages; if none, start title generation asynchronously.
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to list messages: %w", err))
	}

	// Filter out invalid assistant messages that cause 400 errors
	msgs = filterHistory(msgs)

	if len(msgs) == 0 {
		go func() {
			defer logging.RecoverPanic("agent.Run", func() {
				logging.ErrorPersist("panic while generating title")
			})
			titleErr := a.generateTitle(context.Background(), sessionID, content)
			if titleErr != nil {
				logging.ErrorPersist(fmt.Sprintf("failed to generate title: %v", titleErr))
			}
		}()
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to get session: %w", err))
	}
	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}

	userMsg, err := a.createUserMessage(ctx, sessionID, content, attachmentParts)
	if err != nil {
		return a.err(fmt.Errorf("failed to create user message: %w", err))
	}
	// Append the new user message to the conversation history.
	msgHistory := append(msgs, userMsg)

	// Inject matched skills context if skills manager is available
	// The skills summary is already in the provider's system message (via SetSkillsManager)
	// Here we inject the full body content of matched skills based on user input
	if a.skillsManager != nil {
		matches := a.skillsManager.MatchSkills(content)
		if len(matches) > 0 {
			skillContext := a.skillsManager.GetSkillContext(matches)
			if skillContext != "" {
				msgHistory = injectSkillsContext(msgHistory, skillContext)
				logging.Debug("Injected skills context", "skills", len(matches))
			}
		}
	}

	for {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			return a.err(ctx.Err())
		default:
			// Continue processing
		}
		agentMessage, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled)
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			return a.err(fmt.Errorf("failed to process events: %w", err))
		}

		// Handle truncation (max_tokens hit)
		if agentMessage.FinishReason() == message.FinishReasonMaxTokens {
			if len(agentMessage.ToolCalls()) > 0 {
				logging.Info("Truncation detected with tool calls, skipping auto-continuation to avoid API sequence error", "sessionID", sessionID)
			} else {
				logging.Info("Truncation detected (max_tokens), attempting auto-continuation...", "sessionID", sessionID)
				// Add a "continue" message to prompt the model to finish its output
				msgHistory = append(msgHistory, agentMessage, message.Message{
					Role:  message.User,
					Parts: []message.ContentPart{message.TextContent{Text: "continue"}},
				})

				// Get the next part of the response
				nextAgentMessage, nextToolResults, nextErr := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
				if nextErr != nil {
					return a.err(fmt.Errorf("failed to continue response: %w", nextErr))
				}

				// Merge nextAgentMessage into agentMessage
				a.mergeMessages(&agentMessage, nextAgentMessage)
				// Update the tool results if any (merging tool results is complex, usually only the final turn has them)
				if nextToolResults != nil {
					toolResults = nextToolResults
				}

				// After merging, we treat the merged message as the current assistant message
				// and continue the loop to check if we need to call tools or finish.
			}
		}

		if cfg.Debug {
			seqId := (len(msgHistory) + 1) / 2
			toolResultFilepath := logging.WriteToolResultsJson(sessionID, seqId, toolResults)
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", "{}", "filepath", toolResultFilepath)
		} else {
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", toolResults)
		}
		if (agentMessage.FinishReason() == message.FinishReasonToolUse) && toolResults != nil {
			// We are not done, we need to respond with the tool response
			msgHistory = append(msgHistory, agentMessage, *toolResults)
			continue
		}
		return AgentEvent{
			Type:    AgentEventTypeResponse,
			Message: agentMessage,
			Done:    true,
		}
	}
}

// mergeMessages merges the content of 'next' into 'orig'.
// It handles concatenation of TextContent and ToolCall.Input.
func (a *agent) mergeMessages(orig *message.Message, next message.Message) {
	if len(next.Parts) == 0 {
		return
	}

	// 1. Merge TextContent if both end/start with it
	nextText := next.Content().Text
	if nextText != "" {
		orig.AppendContent(nextText)
	}

	// 2. Merge ToolCalls
	origTCs := orig.ToolCalls()
	nextTCs := next.ToolCalls()

	if len(origTCs) > 0 && len(nextTCs) > 0 {
		// If the last tool call of 'orig' is the first of 'next' (by ID or index), merge them
		lastOrigTC := &origTCs[len(origTCs)-1]
		firstNextTC := nextTCs[0]

		// Usually, the model continues the same tool call if it was truncated
		// If IDs match or if the first next TC has no name/ID (just input), we merge
		if lastOrigTC.ID == firstNextTC.ID || (firstNextTC.ID == "" && firstNextTC.Name == "") {
			lastOrigTC.Input += firstNextTC.Input
			lastOrigTC.Finished = firstNextTC.Finished
			// Update the part in 'orig'
			orig.AddToolCall(*lastOrigTC)

			// Append any additional tool calls from 'next'
			if len(nextTCs) > 1 {
				for _, tc := range nextTCs[1:] {
					orig.AddToolCall(tc)
				}
			}
		} else {
			// Just append all new tool calls
			for _, tc := range nextTCs {
				orig.AddToolCall(tc)
			}
		}
	} else if len(nextTCs) > 0 {
		// Just append all new tool calls
		for _, tc := range nextTCs {
			orig.AddToolCall(tc)
		}
	}

	// 3. Update FinishReason from the last message
	if next.IsFinished() {
		orig.AddFinish(next.FinishReason())
	}
}

func (a *agent) createUserMessage(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	parts = append(parts, attachmentParts...)
	return a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
}

func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (message.Message, *message.Message, error) {
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)

	// Filter tools based on permissions mode
	var availableTools []tools.BaseTool
	if a.permissionsMode == types.Plan {
		for _, t := range a.tools {
			if t.IsReadOnly() {
				availableTools = append(availableTools, t)
			}
		}
	} else {
		availableTools = a.tools
	}

	eventChan := a.provider.StreamResponse(ctx, msgHistory, availableTools)

	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{},
		Model: a.provider.Model().ID,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context if needed by tools.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, &assistantMsg, event); processErr != nil {
			a.finishMessage(ctx, &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, nil, processErr
		}
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, nil, ctx.Err()
		}
	}

	toolResults := make([]message.ToolResult, len(assistantMsg.ToolCalls()))
	toolCalls := assistantMsg.ToolCalls()
	for i, toolCall := range toolCalls {
		select {
		case <-ctx.Done():
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			// Make all future tool calls cancelled
			for j := i; j < len(toolCalls); j++ {
				toolResults[j] = message.ToolResult{
					ToolCallID: toolCalls[j].ID,
					Content:    "Tool execution canceled by user",
					IsError:    true,
				}
			}
			goto out
		default:
			// Continue processing
		}

		var tool tools.BaseTool
		for _, availableTool := range availableTools {
			if availableTool.Info().Name == toolCall.Name {
				tool = availableTool
				break
			}
		}

		// Tool not found
		if tool == nil {
			toolResults[i] = message.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
				IsError:    true,
			}
			continue
		}

		toolResult, toolErr := tool.Run(ctx, tools.ToolCall{
			ID:    toolCall.ID,
			Name:  toolCall.Name,
			Input: toolCall.Input,
		})
		if toolErr != nil {
			if errors.Is(toolErr, permission.ErrorPermissionDenied) {
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    "Permission denied",
					IsError:    true,
				}
				for j := i + 1; j < len(toolCalls); j++ {
					toolResults[j] = message.ToolResult{
						ToolCallID: toolCalls[j].ID,
						Content:    "Tool execution canceled by user",
						IsError:    true,
					}
				}
				a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied)
				break
			} else {
				// For any other error, pass the error message back to the LLM
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Tool Execution Error: %s", toolErr.Error()),
					IsError:    true,
				}
				continue
			}
		}
		toolResults[i] = message.ToolResult{
			ToolCallID: toolCall.ID,
			Content:    toolResult.Content,
			Metadata:   toolResult.Metadata,
			IsError:    toolResult.IsError,
		}
	}
out:
	if len(toolResults) == 0 {
		return assistantMsg, nil, nil
	}
	parts := make([]message.ContentPart, 0)
	for _, tr := range toolResults {
		parts = append(parts, tr)
	}
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID, message.CreateMessageParams{
		Role:  message.Tool,
		Parts: parts,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create cancelled tool message: %w", err)
	}

	return assistantMsg, &msg, err
}

func (a *agent) finishMessage(ctx context.Context, msg *message.Message, finishReson message.FinishReason) {
	msg.AddFinish(finishReson)
	_ = a.messages.Update(ctx, *msg)
}

func (a *agent) processEvent(ctx context.Context, sessionID string, assistantMsg *message.Message, event provider.ProviderEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing.
	}

	switch event.Type {
	case provider.EventThinkingDelta:
		assistantMsg.AppendReasoningContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventContentDelta:
		assistantMsg.AppendContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStart:
		assistantMsg.AddToolCall(*event.ToolCall)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseDelta:
		assistantMsg.AppendToolCallInput(event.ToolCall.ID, event.ToolCall.Input)
		tm := time.Unix(assistantMsg.UpdatedAt, 0)
		if time.Since(tm) > 500*time.Millisecond {
			err := a.messages.Update(ctx, *assistantMsg)
			assistantMsg.UpdatedAt = time.Now().Unix()
			return err
		}
	case provider.EventToolUseStop:
		assistantMsg.FinishToolCall(event.ToolCall.ID)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventError:
		if errors.Is(event.Error, context.Canceled) {
			logging.InfoPersist(fmt.Sprintf("Event processing canceled for session: %s", sessionID))
			return context.Canceled
		}
		logging.ErrorPersist(event.Error.Error())
		return event.Error
	case provider.EventComplete:
		assistantMsg.SetToolCalls(event.Response.ToolCalls)
		assistantMsg.AddFinish(event.Response.FinishReason)
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return a.TrackUsage(ctx, sessionID, a.provider.Model(), event.Response.Usage)
	}

	return nil
}

func (a *agent) TrackUsage(ctx context.Context, sessionID string, model models.Model, usage provider.TokenUsage) error {
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)

	sess.Cost += cost
	sess.CompletionTokens = usage.OutputTokens + usage.CacheReadTokens
	sess.PromptTokens = usage.InputTokens + usage.CacheCreationTokens

	_, err = a.sessions.Save(ctx, sess)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func (a *agent) Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error) {
	if a.IsBusy() {
		return models.Model{}, fmt.Errorf("cannot change model while processing requests")
	}

	logging.InfoPersist(fmt.Sprintf("[agent.Update] Called with modelID: %q (%s)", string(modelID), agentName))

	if err := config.SaveActiveModel(context.Background(), modelID); err != nil {
		logging.InfoPersist(fmt.Sprintf("[agent.Update] Failed to save model to DB: %v", err))
		return models.Model{}, fmt.Errorf("failed to update config: %w", err)
	}

	// CreateAgentProviderWithModel will now use config.GetProvider which handles memory storage

	// Use the provided modelID directly instead of reading from database
	// This avoids race conditions and ensures we use the correct model
	provider, err := CreateAgentProviderWithModel(agentName, modelID, a.permissionsMode)
	if err != nil {
		logging.InfoPersist(fmt.Sprintf("[agent.Update] Failed to create provider: %v", err))
		return models.Model{}, fmt.Errorf("failed to create provider for model %s: %w", modelID, err)
	}

	a.provider = provider

	// Also update titleProvider and summarizeProvider to keep them in sync
	// This prevents errors when generating titles or summaries after model switch
	if a.titleProvider != nil {
		titleProvider, err := CreateAgentProviderWithModel(config.AgentTitle, modelID, a.permissionsMode)
		if err != nil {
			logging.InfoPersist(fmt.Sprintf("[agent.Update] Failed to update titleProvider: %v", err))
		} else {
			a.titleProvider = titleProvider
		}
	}
	if a.summarizeProvider != nil {
		summarizeProvider, err := CreateAgentProviderWithModel(config.AgentSummarizer, modelID, a.permissionsMode)
		if err != nil {
			logging.InfoPersist(fmt.Sprintf("[agent.Update] Failed to update summarizeProvider: %v", err))
		} else {
			a.summarizeProvider = summarizeProvider
		}
	}

	logging.InfoPersist(fmt.Sprintf("[agent.Update] Successfully updated to model: %s", a.provider.Model().ID))
	return a.provider.Model(), nil
}

func (a *agent) Summarize(ctx context.Context, sessionID string) error {
	if a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Create a new context with cancellation
	summarizeCtx, cancel := context.WithCancel(ctx)

	// Store the cancel function in activeRequests to allow cancellation
	a.activeRequests.Store(sessionID+"-summarize", cancel)

	go func() {
		defer a.activeRequests.Delete(sessionID + "-summarize")
		defer cancel()
		event := AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Starting summarization...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		// Get all messages from the session
		msgs, err := a.messages.List(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to list messages: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		msgs = filterHistory(msgs)
		summarizeCtx = context.WithValue(summarizeCtx, tools.SessionIDContextKey, sessionID)

		if len(msgs) == 0 {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("no messages to summarize"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Analyzing conversation...",
		}
		a.Publish(pubsub.CreatedEvent, event)

		// Add a system message to guide the summarization
		summarizePrompt := "Provide a detailed but concise summary of our conversation above. Focus on information that would be helpful for continuing the conversation, including what we did, what we're doing, which files we're working on, and what we're going to do next."

		// Create a new message with the summarize prompt
		promptMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
		}

		// Append the prompt to the messages
		msgsWithPrompt := append(msgs, promptMsg)

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Generating summary...",
		}

		a.Publish(pubsub.CreatedEvent, event)

		// Send the messages to the summarize provider
		response, err := a.summarizeProvider.SendMessages(
			summarizeCtx,
			msgsWithPrompt,
			make([]tools.BaseTool, 0),
		)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to summarize: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		summary := strings.TrimSpace(response.Content)
		if summary == "" {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("empty summary returned"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Creating new session...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		oldSession, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to get session: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		// Create a message in the new session with the summary
		msg, err := a.messages.Create(summarizeCtx, oldSession.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: summary},
				message.Finish{
					Reason: message.FinishReasonEndTurn,
					Time:   time.Now().Unix(),
				},
			},
			Model: a.summarizeProvider.Model().ID,
		})
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to create summary message: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		oldSession.SummaryMessageID = msg.ID
		oldSession.CompletionTokens = response.Usage.OutputTokens
		oldSession.PromptTokens = 0
		model := a.summarizeProvider.Model()
		usage := response.Usage
		cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
			model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens)
		oldSession.Cost += cost
		_, err = a.sessions.Save(summarizeCtx, oldSession)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to save session: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
		}

		event = AgentEvent{
			Type:      AgentEventTypeSummarize,
			SessionID: oldSession.ID,
			Progress:  "Summary complete",
			Done:      true,
		}
		a.Publish(pubsub.CreatedEvent, event)
		// Send final success event with the new session ID
	}()

	return nil
}

func CreateAgentProviderWithModel(agentName config.AgentName, modelID models.ModelID, mode types.PermissionsMode) (provider.Provider, error) {
	model, ok := models.SupportedModels[modelID]
	if !ok {
		return nil, fmt.Errorf("model %s not supported", modelID)
	}

	providerCfg, ok := config.GetProvider(model.Provider)
	if !ok {
		return nil, fmt.Errorf("provider %s not found in registry", model.Provider)
	}
	if providerCfg.Disabled {
		return nil, fmt.Errorf("provider %s is not enabled", model.Provider)
	}

	// Debug logging for configuration
	logging.InfoPersist(fmt.Sprintf("[createAgentProviderWithModel] Model: %s, Provider: %s, BaseURL from config: %q, APIKey length: %d",
		model.ID, model.Provider, providerCfg.BaseURL, len(providerCfg.APIKey)))
	maxTokens := model.DefaultMaxTokens
	if agentName == config.AgentTitle {
		maxTokens = 80
	}

	opts := []provider.ProviderClientOption{
		provider.WithAPIKey(providerCfg.APIKey),
		provider.WithProviderBaseURL(providerCfg.BaseURL),
		provider.WithModel(model),
		provider.WithSystemMessage(prompt.GetAgentPrompt(agentName, model.Provider, mode)),
		provider.WithMaxTokens(maxTokens),
	}
	if model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal && model.CanReason {
		opts = append(
			opts,
			provider.WithOpenAIOptions(
				provider.WithReasoningEffort("medium"),
			),
		)
	} else if model.Provider == models.ProviderAnthropic && model.CanReason && agentName == config.AgentCoder {
		opts = append(
			opts,
			provider.WithAnthropicOptions(
				provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn),
			),
		)
	}

	agentProvider, err := provider.NewProvider(
		model.Provider,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create provider: %v", err)
	}

	return agentProvider, nil
}

func createAgentProvider(agentName config.AgentName, mode types.PermissionsMode, extraSystemMessage ...string) (provider.Provider, error) {
	modelID := config.ActiveModel(context.Background(), models.GPT4oMini)
	if agentName == config.AgentTitle {
		modelID = config.ActiveModel(context.Background(), models.GPT4oMini)
	}
	model, ok := models.SupportedModels[modelID]
	if !ok {
		return nil, fmt.Errorf("model %s not supported", modelID)
	}

	providerCfg, ok := config.GetProvider(model.Provider)
	if !ok {
		return nil, fmt.Errorf("provider %s not found in registry", model.Provider)
	}
	if providerCfg.Disabled {
		return nil, fmt.Errorf("provider %s is not enabled", model.Provider)
	}

	// Debug logging for configuration
	logging.InfoPersist(fmt.Sprintf("[createAgentProvider] Model: %s, Provider: %s, BaseURL from config: %q, APIKey length: %d",
		model.ID, model.Provider, providerCfg.BaseURL, len(providerCfg.APIKey)))

	// Build system message with optional extra content (e.g., skills summary)
	systemMsg := prompt.GetAgentPrompt(agentName, model.Provider, mode)
	for _, extra := range extraSystemMessage {
		systemMsg += extra
	}
	maxTokens := model.DefaultMaxTokens
	if agentName == config.AgentTitle {
		maxTokens = 80
	}

	opts := []provider.ProviderClientOption{
		provider.WithAPIKey(providerCfg.APIKey),
		provider.WithProviderBaseURL(providerCfg.BaseURL),
		provider.WithModel(model),
		provider.WithSystemMessage(systemMsg),
		provider.WithMaxTokens(maxTokens),
	}
	if model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal && model.CanReason {
		opts = append(
			opts,
			provider.WithOpenAIOptions(
				provider.WithReasoningEffort("medium"),
			),
		)
	} else if model.Provider == models.ProviderAnthropic && model.CanReason && agentName == config.AgentCoder {
		opts = append(
			opts,
			provider.WithAnthropicOptions(
				provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn),
			),
		)
	}

	agentProvider, err := provider.NewProvider(
		model.Provider,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create provider: %v", err)
	}

	return agentProvider, nil
}

func filterHistory(msgs []message.Message) []message.Message {
	filtered := make([]message.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == message.Assistant {
			hasContent := false
			for _, p := range m.Parts {
				switch v := p.(type) {
				case message.TextContent:
					if v.Text != "" {
						hasContent = true
					}
				case message.ReasoningContent:
					if v.Thinking != "" {
						hasContent = true
					}
				case message.ImageURLContent, message.BinaryContent, message.ToolCall:
					hasContent = true
				}
				if hasContent {
					break
				}
			}
			if !hasContent {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	return filtered
}
