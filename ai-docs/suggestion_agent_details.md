# SuggestionAgent Detailed Documentation

## Overview
SuggestionAgent provides vocabulary suggestions and sentence starters to help English learners respond in conversations. After each AI response, it generates contextual suggestions including a leading sentence and vocabulary options.

## Purpose
The agent helps learners by:
- Providing sentence structure guidance
- Suggesting relevant vocabulary words
- Offering multiple response options
- Adapting to learner's proficiency level

## Structure

```go
type SuggestionAgent struct {
    name        string
    client      client.Client
    level       models.ConversationLevel
    topic       string
    language    string
    model       string
    temperature float64
    maxTokens   int
    config      *utils.SuggestionPromptConfig
}
```

**Fields:**
- `name` - Agent identifier ("SuggestionAgent")
- `client` - OpenRouter API client
- `level` - Current conversation proficiency level
- `topic` - Conversation topic context
- `language` - Target language for translating leading sentences (e.g., "Vietnamese")
- `model` - LLM model name (loaded from `_suggestion_vocab_prompt.yaml`)
- `temperature` - Creativity setting (loaded from YAML, default: 0.7)
- `maxTokens` - Response length limit (loaded from YAML, default: 150)
- `config` - Loaded configuration from `_suggestion_vocab_prompt.yaml`

## Response Format

### SuggestionResponse
```go
type SuggestionResponse struct {
    LeadingSentence string        `json:"leading_sentence"`
    VocabOptions    []VocabOption `json:"vocab_options"`
}

type VocabOption struct {
    Text  string `json:"text"`
    Emoji string `json:"emoji"`
}
```

**Example Response (English):**
```json
{
  "leading_sentence": "You could respond by saying 'I find ... quite ...'",
  "vocab_options": [
    {
      "text": "that really fascinating",
      "emoji": "🤩"
    },
    {
      "text": "it somewhat challenging",
      "emoji": "💪"
    },
    {
      "text": "this surprisingly enjoyable",
      "emoji": "😊"
    }
  ]
}
```

**Example Response (Vietnamese - with language translation):**
```json
{
  "leading_sentence": "Bạn có thể trả lời 'Tôi thấy ... khá ...'",
  "vocab_options": [
    {
      "text": "that really fascinating",
      "emoji": "🤩"
    },
    {
      "text": "it somewhat challenging",
      "emoji": "💪"
    },
    {
      "text": "this surprisingly enjoyable",
      "emoji": "😊"
    }
  ]
}
```

**Note:** The `leading_sentence` is translated to the target language, while `vocab_options` text remains in English for learning purposes.

## Initialization

### NewSuggestionAgent
```go
func NewSuggestionAgent(
    client client.Client,
    level models.ConversationLevel,
    topic string,
    language string,
) *SuggestionAgent
```

**Parameters:**
- `client` - OpenRouter API client
- `level` - Conversation proficiency level (beginner → fluent)
- `topic` - Conversation topic for context
- `language` - Target language for translating instructions (e.g., "Vietnamese", "English")

**Process:**
1. Validates conversation level (defaults to intermediate if invalid)
2. Validates language parameter (defaults to "English" if empty)
3. Loads configuration from `prompts/_suggestion_vocab_prompt.yaml`
4. Extracts LLM settings from config:
   - Model (e.g., "openai/gpt-4o-mini")
   - Temperature (default: 0.7)
   - MaxTokens (default: 150)
5. Returns configured agent with all settings

## Core Methods

### 1. ProcessTask
Main entry point for generating suggestions.

```go
func (sa *SuggestionAgent) ProcessTask(task models.JobRequest) *models.JobResponse
```

**Process:**
1. Extracts last message from task
2. Calls generateSuggestions
3. Returns JobResponse with suggestions

### 2. generateSuggestions
Generates contextual vocabulary suggestions.

```go
func (sa *SuggestionAgent) generateSuggestions(task models.JobRequest) *models.JobResponse
```

**Process:**
1. Extract last AI message from conversation (fallback to default if empty)
2. Build system prompt based on level (creative and flexible guidelines)
3. Create user prompt with AI's message as context
4. Build JSON Schema for structured output (OpenRouter format)
5. Call LLM with `ChatCompletionWithFormat` using strict JSON schema
6. Return validated JSON response

**Prompt Structure:**
- System: Creative, level-specific guidelines emphasizing variety and natural responses
- User: AI's last message + request for diverse, emoji-enhanced suggestions

**Implementation Notes:**
- Uses OpenRouter's [Structured Outputs](https://openrouter.ai/docs/features/structured-outputs) feature
- JSON schema enforces exact format with `strict: true`
- Guarantees valid JSON response without parsing errors
- Each vocab option includes relevant emoji for visual enhancement

### 3. buildSuggestionPrompt
Creates level-specific system prompt from YAML configuration.

```go
func (sa *SuggestionAgent) buildSuggestionPrompt() string
```

**Returns:** System prompt with level-appropriate guidelines

**Process:**
1. Loads base prompt from `_suggestion_vocab_prompt.yaml`
2. Retrieves level-specific guidelines from config
3. Builds complete prompt with guidelines and key principles
4. Falls back to default prompt if config is unavailable

**Configuration Source:** All prompts and guidelines are now loaded from `prompts/_suggestion_vocab_prompt.yaml` for easy maintenance and updates.

**Level-Specific Guidelines:**

#### Beginner
- **Style:** Very simple and fun
- **Vocabulary:** Basic everyday words (1-2 words)
- **Patterns:** Simple sentence structures with clear blanks
- **Emojis:** Clear, obvious matches
- **Variety:** Positive, negative, neutral options
- **Example:** "You can say 'I like ...'"
  - 🐱 "cats"
  - 🍕 "pizza"  
  - 🎵 "music"

#### Elementary
- **Style:** Simple but expressive
- **Vocabulary:** Common phrases (2-3 words)
- **Patterns:** Straightforward with variety
- **Emojis:** Expressive and clear
- **Variety:** Mix different response types
- **Example:** "Try saying 'I enjoy ... because ...'"
  - 🎮 "playing games"
  - 👥 "meeting friends"
  - 📚 "learning new things"

#### Intermediate
- **Style:** Varied and descriptive
- **Vocabulary:** Phrases and expressions (3-4 words)
- **Patterns:** Flexible structures with personality
- **Emojis:** Contextual and meaningful
- **Variety:** Different angles of response
- **Example:** "You could respond 'I find ... quite ...'"
  - 🤩 "that really fascinating"
  - 💪 "it somewhat challenging"
  - 😊 "this surprisingly enjoyable"

#### Upper Intermediate
- **Style:** Sophisticated and nuanced
- **Vocabulary:** Longer phrases with idioms (4-5 words)
- **Patterns:** Complex with variety
- **Emojis:** Nuanced and appropriate
- **Variety:** Multiple perspectives
- **Example:** "Consider saying 'What appeals to me is ...'"
  - 🎨 "the creative aspect"
  - 🧠 "the intellectual challenge"
  - 🤝 "the social dynamics"

#### Advanced
- **Style:** Natural and idiomatic
- **Vocabulary:** Full phrases with collocations (5-6 words)
- **Patterns:** Native-like structures
- **Emojis:** Subtle and appropriate
- **Variety:** Diverse angles and tones
- **Example:** "You might say 'I'm particularly drawn to ...'"
  - 🧩 "the underlying psychological elements"
  - ♟️ "the strategic complexity involved"
  - 🌍 "the cultural significance behind it"

#### Fluent
- **Style:** Authentic and sophisticated
- **Vocabulary:** Complete expressions (6+ words)
- **Patterns:** Natural discourse markers
- **Emojis:** Contextually perfect
- **Variety:** Rich variety in tone and style
- **Example:** "You could articulate 'What I find compelling is ...'"
  - 🏛️ "the way it embodies cultural values"
  - 💭 "how it challenges conventional thinking"
  - 🌐 "the manner it facilitates connection"

### 4. DisplaySuggestions
Formats and displays suggestions in terminal.

```go
func (sa *SuggestionAgent) DisplaySuggestions(jsonResponse string)
```

**Output Format:**
```
💡 Suggestions:
────────────────────────────────────────
📝 You could respond by saying 'I find ... quite ...'

Vocabulary options:
  1. 🤩 that really fascinating
  2. 💪 it somewhat challenging
  3. 😊 this surprisingly enjoyable
────────────────────────────────────────
```

**Features:**
- Cleans JSON response (removes code blocks if present)
- Parses JSON to SuggestionResponse
- Formats with visual separators
- Handles parsing errors gracefully

### 5. ParseSuggestionResponse
Utility function to parse suggestion JSON.

```go
func ParseSuggestionResponse(jsonResponse string) (*SuggestionResponse, error)
```

**Process:**
1. Trim whitespace
2. Remove markdown code blocks if present
3. Parse JSON to SuggestionResponse
4. Return parsed struct or error

**Use Cases:**
- Web API integration
- Testing
- Custom display formatting

## Level Management

### SetLevel
```go
func (sa *SuggestionAgent) SetLevel(level models.ConversationLevel)
```

Updates the agent's conversation level, affecting suggestion complexity.

### GetLevel
```go
func (sa *SuggestionAgent) GetLevel() models.ConversationLevel
```

Returns current conversation level.

## Agent Interface Implementation

### Name
```go
func (sa *SuggestionAgent) Name() string
```
Returns: "SuggestionAgent"

### Capabilities
```go
func (sa *SuggestionAgent) Capabilities() []string
```
Returns:
- vocabulary_suggestion
- response_guidance
- sentence_completion

### CanHandle
```go
func (sa *SuggestionAgent) CanHandle(task string) bool
```

**Handles tasks containing:**
- "suggestion"
- "vocab"
- "help"

### GetDescription
```go
func (sa *SuggestionAgent) GetDescription() string
```
Returns: "Provides vocabulary suggestions and sentence starters to help users respond in conversations"

## Integration Examples

### CLI Integration (ChatbotOrchestrator)
**Status:** ✅ Already integrated

```go
func (co *ChatbotOrchestrator) processUserMessage(userMessage string) {
    conversationJob := models.JobRequest{
        Task:        "conversation",
        UserMessage: userMessage,
    }

    conversationResponse := co.manager.ProcessJob(conversationJob)
    if !conversationResponse.Success {
        utils.PrintError(fmt.Sprintf("Conversation failed: %s", conversationResponse.Error))
        return
    }

    suggestionAgent, exists := co.manager.GetAgent("SuggestionAgent")
    if exists {
        suggestionJob := models.JobRequest{
            Task:        "suggestion",
            UserMessage: conversationResponse.Result,
        }

        suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
        if suggestionResponse.Success {
            sa := suggestionAgent.(*SuggestionAgent)
            sa.DisplaySuggestions(suggestionResponse.Result)
        }
    }
}
```

**Integration Points:**
- After every user message response
- After conversation starter (initial greeting)
- After conversation reset

### Web API Integration
```go
type ChatResponse struct {
    Success     bool               `json:"success"`
    Message     string             `json:"message"`
    Suggestions *SuggestionResponse `json:"suggestions,omitempty"`
}

suggestionJSON := suggestionAgent.ProcessTask(task).Result
suggestions, err := ParseSuggestionResponse(suggestionJSON)
if err == nil {
    response.Suggestions = suggestions
}
```

### Integration with Manager
```go
func (m *AgentManager) RegisterAgents(level models.ConversationLevel, topic string, language string) {
    conversationAgent := NewConversationAgent(m.apiClient, level, topic)
    suggestionAgent := NewSuggestionAgent(m.apiClient, level, topic, language)
    
    m.agents[conversationAgent.Name()] = conversationAgent
    m.agents[suggestionAgent.Name()] = suggestionAgent
}
```

**Status:** ✅ Integrated in production code with language support

## Usage Patterns

### Pattern 1: After Each Response
Display suggestions after every AI response to guide the learner.

```go
for userInput := range inputs {
    conversationResponse := conversationAgent.ProcessTask(task)
    fmt.Println(conversationResponse.Result)
    
    suggestionResponse := suggestionAgent.ProcessTask(suggestionTask)
    suggestionAgent.DisplaySuggestions(suggestionResponse.Result)
}
```

### Pattern 2: On-Demand
Only show suggestions when user requests help.

```go
if userInput == "help" || userInput == "suggest" {
    suggestionResponse := suggestionAgent.ProcessTask(suggestionTask)
    suggestionAgent.DisplaySuggestions(suggestionResponse.Result)
}
```

### Pattern 3: Adaptive
Show suggestions for beginners, hide for advanced learners.

```go
if level <= models.ConversationLevelIntermediate {
    suggestionResponse := suggestionAgent.ProcessTask(suggestionTask)
    suggestionAgent.DisplaySuggestions(suggestionResponse.Result)
}
```

## Best Practices

### Suggestion Quality
- **Exactly 3 options** - Enforced by JSON schema (minItems: 3, maxItems: 3)
- **Contextually relevant** - Based on AI's last message
- **Level-appropriate** - Vocabulary complexity matches learner level
- **Natural language** - Conversational and usable suggestions
- **Emoji-enhanced** - Visual cues that match meaning
- **Diverse responses** - Varied perspectives, tones, and angles
- **Creative variety** - Different types of responses (agreement, curiosity, elaboration)

### Performance
- Keep maxTokens low (150) for fast responses
- Use appropriate temperature (0.7) for variety
- Uses direct ChatCompletion (non-streaming) for immediate results
- Cache suggestions for repeated patterns (optional enhancement)

### Error Handling
- Gracefully handle JSON parsing errors
- Fall back to generic suggestions if needed
- Log errors without breaking flow

### User Experience
- Display suggestions clearly
- Use visual separators
- Number vocabulary options
- Keep formatting consistent

## Testing Considerations

### Unit Tests
- Test JSON parsing with various formats
- Validate level-specific prompts
- Check error handling

### Integration Tests
- Test with real conversation flow
- Validate suggestion relevance
- Check timing and performance

### Example Test Cases
```go
// Test JSON parsing
jsonResponse := `{"leading_sentence":"Try 'I like ...'","vocab_options":["soccer","tennis","golf"]}`
suggestion, err := ParseSuggestionResponse(jsonResponse)

// Test level switching
agent.SetLevel(models.ConversationLevelBeginner)
response := agent.ProcessTask(task)

// Test markdown cleanup
jsonWithMarkdown := "```json\n{...}\n```"
suggestion, err := ParseSuggestionResponse(jsonWithMarkdown)
```

## Current Features (Implemented)

✅ **OpenRouter Structured Outputs** - Guaranteed valid JSON responses
✅ **Emoji Integration** - Visual enhancement for vocabulary
✅ **Creative Variety** - Diverse, flexible suggestions
✅ **Level-Adaptive** - 6 levels with specific guidelines
✅ **Context-Aware** - Based on AI's last message
✅ **Type-Safe** - Strict JSON schema validation
✅ **Multi-Language Support** - Leading sentences translated to target language
✅ **YAML Configuration** - All settings externalized to `_suggestion_vocab_prompt.yaml`
✅ **Template System** - Dynamic prompt building with placeholders
✅ **Configurable LLM Settings** - Model, temperature, and tokens from YAML

## Future Enhancements

### Possible Features
1. ✅ ~~**Multi-language Support**: Show suggestions in learner's native language~~ **IMPLEMENTED** - Leading sentences now translate to target language
2. **Pronunciation Hints**: Phonetic guides for vocabulary (IPA notation)
3. **Example Sentences**: Full example responses for context
4. **Grammar Tips**: Brief grammar explanations
5. **Difficulty Indicators**: Visual markers for advanced vocabulary
6. **Personalization**: Learn and adapt to user's vocabulary preferences
7. **Context History**: Use longer conversation history for better suggestions
8. **Synonym Groups**: Provide related vocabulary clusters
9. **Audio Pronunciation**: Links to audio pronunciation
10. **Usage Frequency**: Indicate how common each phrase is

### API Extensions
```go
type EnhancedSuggestion struct {
    LeadingSentence    string              `json:"leading_sentence"`
    VocabOptions       []EnhancedVocabOption `json:"vocab_options"`
    ExampleSentences   []string            `json:"example_sentences,omitempty"`
    GrammarTip         string              `json:"grammar_tip,omitempty"`
}

type EnhancedVocabOption struct {
    Text          string `json:"text"`
    Emoji         string `json:"emoji"`
    Difficulty    string `json:"difficulty"`      // "basic", "intermediate", "advanced"
    Pronunciation string `json:"pronunciation,omitempty"` // IPA notation
    Frequency     string `json:"frequency,omitempty"`     // "common", "occasional", "rare"
    ExampleUsage  string `json:"example_usage,omitempty"` // Example in context
}
```

## Error Scenarios

### Common Issues
1. **Invalid JSON**: LLM returns malformed JSON
   - Solution: Retry with explicit JSON format instruction

2. **Empty Suggestions**: No vocabulary options provided
   - Solution: Use fallback generic suggestions

3. **Wrong Format**: Different JSON structure returned
   - Solution: Add format validation and normalization

4. **Context Too Long**: Token limit exceeded
   - Solution: Truncate context to last few messages

### Error Messages
- "Failed to parse suggestions" - JSON parsing error
- "Failed to generate suggestions" - LLM call failed
- "Invalid level" - Level validation failed

## Configuration

### Configuration File: `_suggestion_vocab_prompt.yaml`

All SuggestionAgent settings are now centralized in `prompts/_suggestion_vocab_prompt.yaml`:

```yaml
suggestion_agent:
  llm:
    model: "openai/gpt-4o-mini"
    temperature: 0.7
    max_tokens: 150
  
  base_prompt: |
    You are a creative English learning assistant...
    
  user_prompt_template: |
    The AI just said: "{last_message}"
    Topic: {topic}
    Level: {level}
    Target Language: {language}
    
  level_guidelines:
    beginner:
      name: "Beginner"
      description: "Keep it very simple and fun"
      guidelines: [...]
      example_leading: "You can say 'I like ...'"
      example_options: [...]
    # ... other levels
    
  key_principles:
    - "Be creative and varied in your suggestions"
    - "Match the conversation context"
    # ... other principles
```

### Tunable Parameters
```go
model: "openai/gpt-4o-mini"  // LLM model (configurable in YAML)
temperature: 0.7              // Creativity (0.0-1.0, configurable in YAML)
maxTokens: 150                // Response length (configurable in YAML)
language: "Vietnamese"        // Target language for instructions
stream: false                 // Direct response, not streaming
```

### Language Support
The agent now supports multi-language instructions:
- **Leading sentence**: Translated to target language to guide the learner
- **Vocabulary options**: Always remain in English (for learning purposes)
- **Placeholder**: `{language}` in templates is replaced with actual language

**Supported Languages (configurable):**
- Vietnamese (default)
- English
- Spanish
- French
- German
- Japanese
- Korean
- Chinese

### Response Method
- **Method:** `ChatCompletionWithFormat` with OpenRouter Structured Outputs
- **Format:** JSON Schema with strict validation
- **Reference:** [OpenRouter Structured Outputs Documentation](https://openrouter.ai/docs/features/structured-outputs)
- **Reason:** Guarantees valid, parseable JSON responses
- **vs Streaming:** ConversationAgent uses streaming for better UX during longer responses, but SuggestionAgent needs validated structured data

### Prompt Templates
✅ **Now externalized to YAML** - All prompts, guidelines, and settings are stored in `prompts/_suggestion_vocab_prompt.yaml` for easy editing without code changes.

**Benefits:**
- No code changes needed to update prompts
- Easy to maintain and version control
- Consistent with other agent configurations
- Support for template variables: `{last_message}`, `{topic}`, `{level}`, `{language}`

## Integration with Existing System

### Implementation Status

✅ **Completed Integrations:**
1. ✅ Added `ChatCompletionWithFormat` method to Client interface
2. ✅ Implemented OpenRouter Structured Outputs in OpenRouterClient
3. ✅ Created ResponseFormat and JSONSchemaSpec types
4. ✅ SuggestionAgent uses structured outputs with JSON schema
5. ✅ Emoji integration in vocabulary options
6. ✅ Creative, flexible prompt system by level
7. ✅ Registered in AgentManager
8. ✅ Integrated in ChatbotOrchestrator (CLI)
   - Shows after every AI response
   - Shows after conversation starter
   - Shows after reset
9. ✅ Guaranteed valid JSON responses (no parsing errors)

⏳ **Pending Integrations:**
1. ⏳ ChatbotWeb integration (web interface)
2. ⏳ Frontend UI components

### Web Integration
Add to ChatbotWeb:
```go
type ChatResponse struct {
    ...existing fields...
    Suggestions *SuggestionResponse `json:"suggestions,omitempty"`
}
```

Add endpoint:
```go
http.HandleFunc("/api/suggestions", cw.handleGetSuggestions)
```

Update frontend to display suggestions after each AI message.

## Current Implementation Details

### Client Interface Changes
Added structured outputs method to support validated JSON responses:

```go
type Client interface {
    ChatCompletionStream(...)   // Used by ConversationAgent (streaming)
    ChatCompletion(...)         // General non-streaming completion
    ChatCompletionWithFormat(...) // Used by SuggestionAgent (structured)
}
```

### OpenRouter Structured Outputs Integration

#### ResponseFormat Types
```go
type ResponseFormat struct {
    Type       string          `json:"type"`
    JSONSchema *JSONSchemaSpec `json:"json_schema,omitempty"`
}

type JSONSchemaSpec struct {
    Name   string                 `json:"name"`
    Strict bool                   `json:"strict"`
    Schema map[string]interface{} `json:"schema"`
}
```

#### OpenRouterClient Implementation
```go
func (oc *openRouterClient) ChatCompletionWithFormat(
    model string, 
    temperature float64, 
    maxTokens int, 
    messages []models.Message,
    responseFormat *models.ResponseFormat,
) (string, error) {
    reqBody := models.ChatRequest{
        Model:          model,
        Messages:       messages,
        Temperature:    temperature,
        MaxTokens:      maxTokens,
        Stream:         false,
        ResponseFormat: responseFormat,
    }
    // Makes request to OpenRouter with response_format parameter
    // Returns validated JSON matching schema
}
```

#### SuggestionAgent JSON Schema Builder
```go
func (sa *SuggestionAgent) buildResponseFormat() *models.ResponseFormat {
    schema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "leading_sentence": map[string]interface{}{
                "type":        "string",
                "description": "A brief, conversational sentence guiding how to respond",
            },
            "vocab_options": map[string]interface{}{
                "type": "array",
                "items": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "text": map[string]interface{}{
                            "type":        "string",
                            "description": "The vocabulary word or phrase",
                        },
                        "emoji": map[string]interface{}{
                            "type":        "string",
                            "description": "A relevant emoji that matches the meaning",
                        },
                    },
                    "required":             []string{"text", "emoji"},
                    "additionalProperties": false,
                },
                "description": "Exactly 3 diverse vocabulary options with emojis",
                "minItems":    3,
                "maxItems":    3,
            },
        },
        "required":             []string{"leading_sentence", "vocab_options"},
        "additionalProperties": false,
    }

    return &models.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &models.JSONSchemaSpec{
            Name:   "suggestion_response",
            Strict: true,
            Schema: schema,
        },
    }
}
```

#### SuggestionAgent Response Method
```go
func (sa *SuggestionAgent) getResponseWithFormat(
    messages []models.Message, 
    responseFormat *models.ResponseFormat,
) string {
    response, err := sa.client.ChatCompletionWithFormat(
        sa.model, 
        sa.temperature, 
        sa.maxTokens, 
        messages,
        responseFormat,
    )
    if err != nil {
        utils.PrintError(fmt.Sprintf("Failed to get suggestion response: %v", err))
        return ""
    }
    return response
}
```

**Benefits of Structured Outputs:**
- **Guaranteed valid JSON** - No parsing errors or malformed responses
- **Type-safe responses** - Schema enforces exact structure
- **No hallucinated fields** - Only returns specified properties
- **Faster processing** - No need for validation or cleanup
- **Emoji integration** - Structured support for emoji in responses
- **Immediate results** - Non-streaming for quick suggestions
- **Reduced latency** - Complete validated response at once

### Usage in Production

**ChatbotOrchestrator Flow:**
1. User sends message
2. ConversationAgent processes (streaming response)
3. Display conversation response
4. SuggestionAgent generates suggestions (direct response)
5. Display suggestions immediately

**Example Output (with Vietnamese language setting):**
```
💬 Responding...
What sports do you enjoy most?

🌐 Vietnamese Translation:
──────────────────────────
🇻🇳 Bạn thích môn thể thao nào nhất?
──────────────────────────

💡 Suggestions:
────────────────────────────────────────
📝 Bạn có thể trả lời 'Tôi thích ... bởi vì ...'

Vocabulary options:
  1. ⚽ playing soccer with friends
  2. 🏀 watching basketball games
  3. 🧘 doing yoga in the morning
────────────────────────────────────────
```

**Key Features in Output:**
- Leading sentence guides the response structure (translated to Vietnamese)
- Each vocab option has a relevant emoji
- Vocabulary options remain in English (for learning)
- Options are diverse (different activities, contexts)
- Natural, conversational suggestions
- Adapted to intermediate level (3-4 word phrases)
- Multi-language instruction support

