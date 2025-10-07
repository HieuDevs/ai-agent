# ConversationAgent Detailed Documentation

## Overview
ConversationAgent is the core agent responsible for handling English conversation practice. It manages conversation context, generates appropriate responses based on user proficiency level, and maintains conversation history.

## Structure

```go
type ConversationAgent struct {
    name        string
    model       string
    temperature float64
    maxTokens   int
    Topic       string
    client      client.Client
    level       models.ConversationLevel
    history     *services.ConversationHistoryManager
}
```

**Fields:**
- `name` - Agent identifier ("ConversationAgent")
- `model` - LLM model name (e.g., "openai/gpt-4o-mini")
- `temperature` - LLM temperature setting (creativity level)
- `maxTokens` - Maximum response length
- `Topic` - Conversation topic (e.g., "sports", "music")
- `client` - OpenRouter API client
- `level` - Current conversation proficiency level
- `history` - Conversation history manager service

## Initialization

### NewConversationAgent
```go
func NewConversationAgent(
    client client.Client,
    level models.ConversationLevel,
    topic string,
    history *services.ConversationHistoryManager,
) *ConversationAgent
```

**Process:**
1. Validates conversation level (defaults to intermediate if invalid)
2. Loads LLM settings from topic prompt file using `utils.GetLLMSettingsFromLevel`
3. Uses provided conversation history manager
4. Returns configured agent

**LLM Settings:**
Loaded from YAML prompt file per level:
- `model` - Which LLM to use
- `temperature` - Response creativity (0.0-1.0)
- `max_tokens` - Response length limit

## Core Methods

### 1. ProcessTask
Main entry point for handling conversation tasks.

```go
func (ca *ConversationAgent) ProcessTask(task models.JobRequest) *models.JobResponse
```

**Logic:**
- If `task.UserMessage` is empty → Generate conversation starter
- Otherwise → Generate conversational response

**Returns:**
- `JobResponse` with success status and response text

### 2. generateConversationStarter
Creates initial conversation message.

```go
func (ca *ConversationAgent) generateConversationStarter() *models.JobResponse
```

**Process:**
1. Loads "starter" prompt from YAML file using `GetLevelSpecificPrompt`
2. Adds to conversation history via `ca.history.AddToHistory`
3. Prints starter message to console
4. Returns as JobResponse

**Example Starters by Level:**
- Beginner: "Hi! Let's talk about sports!"
- Intermediate: "What sports do you enjoy most?"
- Advanced: "How do you think sports culture differs globally?"

### 3. generateConversationalResponse
Generates contextual response to user input.

```go
func (ca *ConversationAgent) generateConversationalResponse(
    task models.JobRequest,
    model string,
    temperature float64,
    maxTokens int,
) *models.JobResponse
```

**Process:**
1. Load level-specific conversational prompt using `GetLevelSpecificPrompt`
2. Build message context:
   - System message with conversational guidelines
   - Full conversation history from `ca.history.GetConversationHistory()`
   - Current user message
3. Get streaming response from LLM using `getStreamingResponse`
4. Add user message and response to history via `ca.history.AddToHistory`
5. Return JobResponse

**Context Window:**
- Uses full conversation history (managed by ConversationHistoryManager)
- History manager handles message limits and sliding windows
- Maintains conversation coherence

### 4. getStreamingResponse
Handles LLM streaming and display.

```go
func (ca *ConversationAgent) getStreamingResponse(
    messages []models.Message,
    prefix string,
    model string,
    temperature float64,
    maxTokens int,
) string
```

**Process:**
1. Create channels for streaming and completion
2. Start goroutine for ChatCompletionStream
3. Print chunks as they arrive
4. On completion, show Vietnamese translation
5. Return full response text

**Features:**
- Real-time response display
- Automatic translation after completion
- Clean terminal output

## History Management

History management is now handled by the `ConversationHistoryManager` service:

### ConversationHistoryManager Integration
- `ca.history.AddToHistory(role, content)` - Add messages to history
- `ca.history.GetConversationHistory()` - Get full conversation history
- `ca.history.GetConversationStats()` - Get conversation statistics
- `ca.history.ResetConversation()` - Clear conversation history

**Benefits:**
- Centralized history management
- Consistent across all agents
- Better separation of concerns
- Enhanced features like statistics and export

## Level Management

### SetLevel
```go
func (ca *ConversationAgent) SetLevel(level models.ConversationLevel)
```

**Process:**
1. Validates level is valid
2. Updates agent's level
3. Prints success message

**Note:** Does not reset history or reload LLM settings. Settings are loaded at initialization.

### GetLevel
```go
func (ca *ConversationAgent) GetLevel() models.ConversationLevel
```

**Returns:** Current conversation level

### GetLevelSpecificCapabilities
```go
func (ca *ConversationAgent) GetLevelSpecificCapabilities() []string
```

**Base Capabilities:**
- english_conversation
- teaching_response
- conversation_starter
- contextual_responses
- level_appropriate_challenge

**Level-Specific Additions:**

**Beginner:**
- basic_vocabulary
- simple_grammar
- patient_coaching

**Elementary:**
- structured_learning
- confidence_building

**Intermediate:**
- complex_discussion
- advanced_grammar

**Upper Intermediate:**
- sophisticated_discussion
- nuanced_language

**Advanced:**
- native_level_interaction
- critical_thinking

**Fluent:**
- authentic_conversation
- expert_debate

## Statistics and Metrics

### Statistics and History Access
Statistics and history are now accessed through the ConversationHistoryManager:

```go
stats := ca.history.GetConversationStats()
history := ca.history.GetConversationHistory()
```

**Returns:**
```go
{
    "total_messages": 10,
    "user_messages": 5,
    "bot_messages": 5
}
```

**Use Cases:**
- Display progress to user
- Track engagement metrics
- Analyze conversation patterns
- Export conversation data

## Translation Integration

### showVietnameseTranslation
```go
func (ca *ConversationAgent) showVietnameseTranslation(text string)
```

**Process:**
1. Validates text is not empty
2. Calls translation service
3. Displays formatted translation
4. Handles errors gracefully

**Output Format:**
```
🌐 Vietnamese Translation:
──────────────────────────
🇻🇳 Bạn có thích thể thao không?
──────────────────────────
```

## Prompt System

### getLevelSpecificPrompt
```go
func getLevelSpecificPrompt(
    path string,
    level models.ConversationLevel,
    promptType string,
) string
```

**Parameters:**
- `path` - Path to YAML prompt file
- `level` - Conversation level
- `promptType` - Type of prompt ("starter", "conversational", "evaluation")

**Process:**
1. Load prompt from YAML file
2. If error, fallback to "intermediate" level
3. Return formatted prompt string

**Prompt Types:**

**starter:**
Initial conversation message

**conversational:**
System prompt for response generation, includes:
- Role description
- Personality traits
- Response guidelines
- Level-appropriate language instructions

## Agent Interface Implementation

### Name
```go
func (ca *ConversationAgent) Name() string
```
Returns: "ConversationAgent"

### Capabilities
```go
func (ca *ConversationAgent) Capabilities() []string
```
Returns base capability list (without level-specific)

### CanHandle
```go
func (ca *ConversationAgent) CanHandle(task string) bool
```

**Checks if task contains:**
- "conversation"
- "chat"
- "talk"

### GetDescription
```go
func (ca *ConversationAgent) GetDescription() string
```
Returns: "Handles English conversation with learners, providing appropriate responses for practice"

## Best Practices

### Context Management
- Keep history window small (6 messages for context)
- Limit total history to 20 messages
- Include system prompt with each request

### Level Appropriateness
- Load level-specific prompts
- Configure LLM settings per level
- Adjust vocabulary and complexity

### Response Quality
- Use streaming for better UX
- Provide translations for learning
- Maintain conversation flow

### Error Handling
- Fallback to intermediate level on invalid settings
- Handle empty responses gracefully
- Log errors without breaking flow

## Integration Points

### Client Interface
Requires `client.Client` implementation with:
- `ChatCompletionStream` method for streaming responses

### Models Package
Uses types from `work-flows/models`:
- `Message` - Conversation message structure
- `MessageRole` - Role enum (user/assistant/system)
- `ConversationLevel` - Level enum
- `JobRequest` - Task request structure
- `JobResponse` - Task response structure

### Utils Package
Uses utilities:
- `GetFullPrompt` - Load prompts from YAML
- `GetLLMSettingsFromLevel` - Extract LLM config
- `GetPromptsDir` - Get prompts directory path
- `PrintInfo/PrintError/PrintSuccess` - Colored console output

### Services Package
- `TranslateToVietnamese` - Translation service integration

