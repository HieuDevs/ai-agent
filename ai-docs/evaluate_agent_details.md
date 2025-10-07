# EvaluateAgent Detailed Documentation

## Overview
EvaluateAgent provides constructive feedback on learner responses by evaluating **relevance first**, then grammar, vocabulary, sentence structure, and context appropriateness. It helps English learners understand their mistakes and improve their language skills through detailed, encouraging feedback.

## Purpose
The agent helps learners by:
- **PRIORITY:** Checking if response is relevant to AI's message/question
- Evaluating response quality against level-appropriate standards
- Identifying grammar, vocabulary, and structural errors
- Providing constructive, encouraging feedback
- Offering corrected versions of responses
- Adapting evaluation criteria to learner's proficiency level

## Critical Evaluation Rule
**Relevance is the PRIMARY criterion.** If a response doesn't relate to or address the AI's message, it automatically receives "needs_improvement" status, regardless of grammar quality. Irrelevant responses CANNOT be rated as "excellent" or "good".

### Relevance Acceptance (Implicit Answers)
- Accept implicit yes/no answers that include a preference, detail, or example as relevant.
- Do NOT require explicit tokens like "Yes/No" to consider a response relevant.
- Elliptical but clear replies are relevant if they directly imply an answer.

Examples:
- AI: "Do you like music?" / User: "I like pop music." → Relevant (implicit YES). Evaluate quality, not relevance.
- AI: "Do you like music?" / User: "Pop music." → Relevant if it clearly implies preference; evaluate quality.

## Structure

```go
type EvaluateAgent struct {
    name        string
    client      client.Client
    level       models.ConversationLevel
    topic       string
    language    string
    model       string
    temperature float64
    maxTokens   int
    config      *utils.EvaluatePromptConfig
}
```

**Fields:**
- `name` - Agent identifier ("EvaluateAgent")
- `client` - OpenRouter API client
- `level` - Current conversation proficiency level
- `topic` - Conversation topic context
- `language` - Target language for feedback (e.g., "Vietnamese", "English")
- `model` - LLM model name (loaded from `_evaluate_prompt.yaml`)
- `temperature` - Consistency setting (loaded from YAML, default: 0.3)
- `maxTokens` - Response length limit (loaded from YAML, default: 500)
- `config` - Loaded configuration from `_evaluate_prompt.yaml`

## Response Format

### EvaluationResponse
```go
type EvaluationResponse struct {
    Status          string `json:"status"`
    ShortDescription string `json:"short_description"`
    LongDescription  string `json:"long_description"`
    Correct          string `json:"correct"`
}
```

**Fields:**
- `Status` - Evaluation level: "excellent", "good", or "needs_improvement"
- `ShortDescription` - Brief encouraging feedback (translated to target language)
 - `LongDescription` - Encouragement-only for "excellent"/"good" (no tags in this case). For "needs_improvement", provide detailed corrections (translated to target language):
   - Use `<err>...</err>` to mark error spans in the user's text
   - Use `<b>...</b>` to highlight correct forms or key points
   - If there are no errors, do not use any tags
- `Correct` - Always provides an example response that properly addresses the AI's message (e.g., if AI asked a question, answer it; if AI made a statement, provide an appropriate reply). For "excellent"/"good", shows the user's original (if already good) or a refined version. For "needs_improvement", shows corrected grammar that properly responds to the AI. This field is always in English only.

**Example Response (Vietnamese):**
```json
{
  "status": "good",
  "short_description": "Câu trả lời của bạn khá tốt! Bạn đã diễn đạt ý tưởng rõ ràng.",
  "long_description": "Rất tốt! Hãy tiếp tục giữ phong độ và luyện tập thêm để diễn đạt tự nhiên hơn.",
  "correct": "I have been playing soccer for 5 years."
}
```

**Example Response (Excellent level):**
```json
{
  "status": "excellent",
  "short_description": "Xuất sắc! Câu của bạn hoàn toàn chính xác.",
  "long_description": "Bạn đã sử dụng <b>thì hiện tại hoàn thành tiếp diễn</b> một cách chính xác, cấu trúc câu tự nhiên, và từ vựng phù hợp với ngữ cảnh. Rất tốt!",
  "correct": "I have been playing soccer for 5 years."
}
```

## Evaluation Levels

### Excellent ✨
- **MUST be relevant** to AI's message/question
- Perfect or near-perfect response with natural English
- Appropriate grammar and vocabulary for level
- Clear communication with proper structure
- No significant errors

### Good 👍
- **MUST be relevant** to AI's message/question
- Solid response that communicates effectively
- Minor issues that don't impede understanding
- Most grammar and vocabulary used correctly
- Room for improvement in specific areas

### Needs Improvement 📚
- **Irrelevant/off-topic response** (regardless of grammar quality) **OR**
- Noticeable errors affecting clarity or naturalness
- Grammar, vocabulary, or structural issues
- Communication is impeded
- Requires correction and practice

## Initialization

### NewEvaluateAgent
```go
func NewEvaluateAgent(
    client client.Client,
    level models.ConversationLevel,
    topic string,
    language string,
) *EvaluateAgent
```

**Parameters:**
- `client` - OpenRouter API client
- `level` - Conversation proficiency level (beginner → fluent)
- `topic` - Conversation topic for context
- `language` - Target language for feedback translation (e.g., "Vietnamese", "English")

**Process:**
1. Validates conversation level (defaults to intermediate if invalid)
2. Validates language parameter (defaults to "English" if empty)
3. Loads configuration from `prompts/_evaluate_prompt.yaml`
4. Extracts LLM settings from config:
   - Model (e.g., "openai/gpt-4o-mini")
   - Temperature (default: 0.3 for consistency)
   - MaxTokens (default: 500 for detailed feedback)
5. Returns configured agent with all settings

## Core Methods

### 1. ProcessTask
Main entry point for generating evaluation.

```go
func (ea *EvaluateAgent) ProcessTask(task models.JobRequest) *models.JobResponse
```

**Process:**
1. Extracts user message and AI context from task
2. Calls generateEvaluation
3. Returns JobResponse with evaluation

### 2. generateEvaluation
Generates detailed evaluation of user response.

```go
func (ea *EvaluateAgent) generateEvaluation(task models.JobRequest) *models.JobResponse
```

**Process:**
1. Extract user message and AI's previous message from task
2. Validate user message is not empty
3. Build system prompt based on level (evaluation criteria and guidelines)
4. Create user prompt with both messages and context
5. Build JSON Schema for structured output (OpenRouter format)
6. Call LLM with `ChatCompletionWithFormat` using strict JSON schema
7. Return validated JSON response

**Prompt Structure:**
- System: Level-specific evaluation criteria and guidelines with **relevance-first priority**
- User: User's response + AI's question/context + topic + level + language
- **Evaluation Instructions:** 
  - **STEP 1:** Check relevance first (is response related to AI's message?)
  - Accept implicit yes/no answers (with details/preferences) as relevant; do not require explicit "Yes/No".
  - **STEP 2:** Evaluate quality only if relevant (grammar, vocabulary, structure)
  - In feedback: wrap errors with `<err>...</err>` and use `<b>...</b>` for highlights; if no errors, use no tags

**Implementation Notes:**
- Uses OpenRouter's [Structured Outputs](https://openrouter.ai/docs/features/structured-outputs) feature
- JSON schema enforces exact format with `strict: true`
- Guarantees valid JSON response without parsing errors
- **Relevance is the PRIMARY criterion** - irrelevant = needs_improvement (regardless of grammar)
- Feedback translated to target language for better understanding
- Lower temperature (0.3) ensures consistent evaluation
- Returns error if user message is empty

### 3. buildEvaluatePrompt
Creates level-specific system prompt from YAML configuration.

```go
func (ea *EvaluateAgent) buildEvaluatePrompt() string
```

**Returns:** System prompt with level-appropriate evaluation criteria

**Process:**
1. Loads base prompt from `_evaluate_prompt.yaml`
2. Retrieves level-specific guidelines and criteria from config
3. Builds complete prompt with guidelines and key principles
4. Falls back to default prompt if config is unavailable

**Configuration Source:** All prompts, guidelines, and criteria are loaded from `prompts/_evaluate_prompt.yaml` for easy maintenance.

**Level-Specific Evaluation Criteria:**

#### Beginner
- **Style:** Very forgiving, focus on encouragement
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND simple, clear sentence with correct basic grammar
  - Good: **Relevant** AND communicates idea despite minor errors (articles, plurals)
  - Needs Improvement: **Irrelevant** OR major verb errors or incomprehensible meaning
- **Focus:** 1-2 key improvements, very simple explanations
- **Approach:** Check relevance first, then praise effort and basic communication

#### Elementary
- **Style:** Encouraging but attentive to basic patterns
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND clear sentences with correct basic tenses and structure
  - Good: **Relevant** AND mostly correct with minor tense or article issues
  - Needs Improvement: **Irrelevant** OR incorrect tense usage or confusing structure
- **Focus:** 2-3 improvement areas with clear examples
- **Approach:** Check relevance first, then basic sentence structure (SVO), simple past/present tenses

#### Intermediate
- **Style:** Balanced - encourage progress while addressing errors
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND natural expression with varied structures and appropriate vocabulary
  - Good: **Relevant** AND good communication with some preposition/collocation issues
  - Needs Improvement: **Irrelevant** OR multiple grammar errors or awkward phrasing affecting clarity
- **Focus:** 3-4 improvement areas with explanations
- **Approach:** Check relevance first, then expect varied sentence structures and appropriate vocabulary

#### Upper Intermediate
- **Style:** Higher standards - expect natural expression
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND sophisticated expression with natural flow and precise vocabulary
  - Good: **Relevant** AND strong communication with minor style or collocation issues
  - Needs Improvement: **Irrelevant** OR unnatural phrasing, wrong collocations, or grammar mistakes
- **Focus:** 4-5 improvement areas with nuance explanations
- **Approach:** Check relevance first, then complex structures, advanced vocabulary, subtle grammar

#### Advanced
- **Style:** Near-native standards expected
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND native-like expression with idioms and natural collocations
  - Good: **Relevant** AND very good with minor non-native patterns
  - Needs Improvement: **Irrelevant** OR unnatural collocations or inappropriate register
- **Focus:** Subtle improvements and sophisticated alternatives
- **Approach:** Check relevance first, then expect idiomatic expressions, natural collocations, appropriate register

#### Fluent
- **Style:** Native-level standards
- **FIRST CHECK:** Is it relevant to AI's message?
- **Criteria:**
  - Excellent: **Relevant** AND indistinguishable from native speaker with elegant expression
  - Good: **Relevant** AND nearly native with very subtle non-native elements
  - Needs Improvement: **Irrelevant** OR noticeable non-native patterns or style issues
- **Focus:** Refinements, sophistication, and elegance
- **Approach:** Check relevance first, then expect completely natural expression with subtle style nuances

### 4. DisplayEvaluation
Formats and displays evaluation in terminal.

```go
func (ea *EvaluateAgent) DisplayEvaluation(jsonResponse string)
```

**Output Format:**
```
📊 Evaluation:
────────────────────────────────────────
👍 Status: GOOD

💬 Câu trả lời của bạn khá tốt! Bạn đã diễn đạt ý tưởng rõ ràng.

📖 Detailed Feedback:
Bạn đã sử dụng cấu trúc câu tốt. Tuy nhiên, có một lỗi nhỏ về thì: 
bạn nên dùng <b>"have been playing"</b> thay vì <b>"am playing"</b> 
vì bạn đang nói về một hành động bắt đầu từ quá khứ và vẫn tiếp tục 
đến hiện tại.

✅ Corrected: I have been playing soccer for 5 years.
────────────────────────────────────────
```

**Features:**
- Status emoji based on evaluation level (✨/👍/📚)
- Cleans JSON response (removes code blocks if present)
- Parses JSON to EvaluationResponse
- Formats with visual separators
- Shows corrected version for all evaluations
- Handles parsing errors gracefully

### 5. ParseEvaluationResponse
Utility function to parse evaluation JSON.

```go
func ParseEvaluationResponse(jsonResponse string) (*models.EvaluationResponse, error)
```

**Process:**
1. Trim whitespace
2. Remove markdown code blocks if present
3. Parse JSON to EvaluationResponse
4. Return parsed struct or error

**Use Cases:**
- Web API integration
- Testing
- Custom display formatting
- Integration with other agents

## Level Management

### SetLevel
```go
func (ea *EvaluateAgent) SetLevel(level models.ConversationLevel)
```

Updates the agent's conversation level, affecting evaluation criteria.

### GetLevel
```go
func (ea *EvaluateAgent) GetLevel() models.ConversationLevel
```

Returns current conversation level.

## Agent Interface Implementation

### Name
```go
func (ea *EvaluateAgent) Name() string
```
Returns: "EvaluateAgent"

### Capabilities
```go
func (ea *EvaluateAgent) Capabilities() []string
```
Returns:
- response_evaluation
- grammar_checking
- feedback_provision

### CanHandle
```go
func (ea *EvaluateAgent) CanHandle(task string) bool
```

**Handles tasks containing:**
- "evaluate"
- "check"
- "feedback"

### GetDescription
```go
func (ea *EvaluateAgent) GetDescription() string
```
Returns: "Evaluates learner responses and provides constructive feedback on grammar, vocabulary, and structure"

## Integration Examples

### CLI Integration (ChatbotOrchestrator)
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

    evaluateAgent, exists := co.manager.GetAgent("EvaluateAgent")
    if exists {
        evaluateJob := models.JobRequest{
            Task:          "evaluate",
            UserMessage:   userMessage,
            LastAIMessage: conversationResponse.Result,
        }

        evaluateResponse := evaluateAgent.ProcessTask(evaluateJob)
        if evaluateResponse.Success {
            ea := evaluateAgent.(*EvaluateAgent)
            ea.DisplayEvaluation(evaluateResponse.Result)
        }
    }
}
```

**Integration Points:**
- After every user message (before showing suggestions)
- Optional: on-demand when user asks for feedback
- Optional: after conversation completion for summary

### Web API Integration
```go
type ChatResponse struct {
    Success    bool                `json:"success"`
    Message    string              `json:"message"`
    Evaluation *EvaluationResponse `json:"evaluation,omitempty"`
}

evaluationJSON := evaluateAgent.ProcessTask(task).Result
evaluation, err := ParseEvaluationResponse(evaluationJSON)
if err == nil {
    response.Evaluation = evaluation
}
```

### Integration with Manager
```go
func (m *AgentManager) RegisterAgents(level models.ConversationLevel, topic string, language string) {
    conversationAgent := NewConversationAgent(m.apiClient, level, topic)
    suggestionAgent := NewSuggestionAgent(m.apiClient, level, topic, language)
    evaluateAgent := NewEvaluateAgent(m.apiClient, level, topic, language)
    
    m.agents[conversationAgent.Name()] = conversationAgent
    m.agents[suggestionAgent.Name()] = suggestionAgent
    m.agents[evaluateAgent.Name()] = evaluateAgent
}
```

## Usage Patterns

### Pattern 1: After Each User Response
Evaluate every user message to provide continuous feedback.

```go
for userInput := range inputs {
    evaluateResponse := evaluateAgent.ProcessTask(evaluateTask)
    evaluateAgent.DisplayEvaluation(evaluateResponse.Result)
    
    conversationResponse := conversationAgent.ProcessTask(conversationTask)
    fmt.Println(conversationResponse.Result)
}
```

### Pattern 2: On-Demand Feedback
Only evaluate when user requests feedback.

```go
if userInput == "check" || userInput == "feedback" {
    evaluateResponse := evaluateAgent.ProcessTask(evaluateTask)
    evaluateAgent.DisplayEvaluation(evaluateResponse.Result)
}
```

### Pattern 3: Adaptive Evaluation
Show detailed evaluation for beginners, brief for advanced learners.

```go
if level <= models.ConversationLevelIntermediate {
    evaluateResponse := evaluateAgent.ProcessTask(evaluateTask)
    evaluateAgent.DisplayEvaluation(evaluateResponse.Result)
} else {
    if evaluation.Status == "needs_improvement" {
        evaluateAgent.DisplayEvaluation(evaluateResponse.Result)
    }
}
```

## Best Practices

### Evaluation Quality
- **Encouraging tone** - Always start with positive feedback
- **Specific errors** - Don't just say "grammar error", explain what's wrong
- **Level-appropriate** - Adjust criteria to learner's proficiency
- **Constructive feedback** - Focus on improvement, not criticism
- **Use highlights** - `<b>tags</b>` to emphasize specific words/errors
- **Provide corrections** - Show the right way to say it
- **Actionable advice** - Make feedback clear and usable

### Performance
- Lower temperature (0.3) for consistent, reliable evaluation
- Moderate maxTokens (500) for detailed but concise feedback
- Uses direct ChatCompletion (non-streaming) for complete evaluation
- Cache configuration for faster processing

### Error Handling
- Gracefully handle JSON parsing errors
- Fall back to basic evaluation if config unavailable
- Log errors without breaking flow
- Validate required fields (user message)

### User Experience
- Display evaluation clearly with emojis
- Use visual separators for readability
- Show corrected version prominently
- Keep feedback encouraging and constructive
- In explanations: wrap error spans with `<err>...</err>` and use `<b>...</b>` for highlights; if no errors, use no tags

## Testing Considerations

### Unit Tests
- Test JSON parsing with various formats
- Validate level-specific criteria
- Check error handling for missing fields
- Test prompt building from config

### Integration Tests
- Test with real conversation flow
- Validate evaluation accuracy
- Check timing and performance
- Test multi-language feedback

### Example Test Cases
```go
// Test JSON parsing
jsonResponse := `{"status":"good","short_description":"Well done!","long_description":"Minor error in <b>tense</b>","correct":"I have been playing"}`
evaluation, err := ParseEvaluationResponse(jsonResponse)

// Test level switching
agent.SetLevel(models.ConversationLevelBeginner)
response := agent.ProcessTask(task)

// Test evaluation criteria
beginnerTask := models.JobRequest{UserMessage: "I play soccer", LastAIMessage: "What do you like?"}
evaluation := agent.ProcessTask(beginnerTask)
```

## Current Features (Implemented)

✅ **OpenRouter Structured Outputs** - Guaranteed valid JSON responses
✅ **Three Evaluation Levels** - excellent/good/needs_improvement
✅ **Multi-Language Feedback** - Feedback translated to target language
✅ **Highlight Support** - `<b>tags</b>` for emphasizing errors/corrections
✅ **Level-Adaptive Criteria** - 6 proficiency levels with specific standards
✅ **Context-Aware** - Considers AI's question and user's response
✅ **Type-Safe** - Strict JSON schema validation
✅ **YAML Configuration** - All settings externalized to `_evaluate_prompt.yaml`
✅ **Configurable LLM Settings** - Model, temperature, and tokens from YAML
✅ **Constructive Feedback** - Encouraging tone with specific improvements

## Future Enhancements

### Possible Features
1. **Progress Tracking**: Track improvement over time per user
2. **Common Mistakes**: Identify patterns in errors for targeted practice
3. **Grammar Rules**: Link to specific grammar rule explanations
4. **Example Sentences**: Provide multiple example corrections
5. **Pronunciation Feedback**: Evaluate pronunciation if audio input available
6. **Comparative Analysis**: Compare to native speaker responses
7. **Detailed Metrics**: Scores for grammar, vocabulary, fluency separately
8. **Learning Resources**: Suggest specific exercises for improvement areas
9. **Peer Comparison**: Anonymous comparison to others at same level
10. **Achievement Badges**: Gamification for consistent improvement

### API Extensions
```go
type EnhancedEvaluation struct {
    Status           string              `json:"status"`
    ShortDescription string              `json:"short_description"`
    LongDescription  string              `json:"long_description"`
    Correct          string              `json:"correct"`
    DetailedScores   DetailedScores      `json:"detailed_scores,omitempty"`
    GrammarRules     []GrammarReference  `json:"grammar_rules,omitempty"`
    Examples         []string            `json:"examples,omitempty"`
    Resources        []LearningResource  `json:"resources,omitempty"`
}

type DetailedScores struct {
    Grammar    float64 `json:"grammar"`      // 0-100
    Vocabulary float64 `json:"vocabulary"`   // 0-100
    Structure  float64 `json:"structure"`    // 0-100
    Fluency    float64 `json:"fluency"`      // 0-100
    Overall    float64 `json:"overall"`      // 0-100
}

type GrammarReference struct {
    Rule        string `json:"rule"`
    Explanation string `json:"explanation"`
    Link        string `json:"link,omitempty"`
}

type LearningResource struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    URL         string `json:"url,omitempty"`
}
```

## Error Scenarios

### Common Issues
1. **No User Message**: Empty user message to evaluate
   - Solution: Return error response asking for message

2. **Invalid JSON**: LLM returns malformed JSON (rare with structured outputs)
   - Solution: Retry with explicit format instruction

3. **Wrong Status**: Status value not in enum (prevented by schema)
   - Solution: Schema validation ensures only valid values

4. **Missing Context**: No AI message for context
   - Solution: Proceed with evaluation but note missing context

### Error Messages
- "No user message to evaluate" - Empty user input
- "Failed to generate evaluation" - LLM call failed
- "Failed to parse evaluation" - JSON parsing error (rare)
- "Invalid level" - Level validation failed

## Configuration

### Configuration File: `_evaluate_prompt.yaml`

All EvaluateAgent settings are centralized in `prompts/_evaluate_prompt.yaml`:

```yaml
evaluate_agent:
  llm:
    model: "openai/gpt-4o-mini"
    temperature: 0.3
    max_tokens: 500
  
  base_prompt: |
    You are an expert English learning evaluator...
    
  user_prompt_template: |
    Evaluate this learner's response:
    User Response: "{user_message}"
    AI Question/Context: "{ai_message}"
    Topic: {topic}
    Level: {level}
    Target Language: {language}
    
    Return:
    1) status (excellent|good|needs_improvement)
    2) short_description (in {language})
    3) long_description (in {language})
    4) correct (English only, never translate)
    
  level_guidelines:
    beginner:
      name: "Beginner"
      description: "Very forgiving, focus on encouragement"
      guidelines: [...]
      criteria:
        excellent: "..."
        good: "..."
        needs_improvement: "..."
    # ... other levels
    
  key_principles:
    - "Always be encouraging and constructive"
    - "Highlight what was done well before errors"
    # ... other principles
    - "Accept implicit yes/no answers with details as relevant; do not require explicit 'Yes/No'"
    - "'correct' must be strictly in English only"

## Clarification for Music Preference Scenario
If AI asks: "Do you like music?" and the user replies: "I like pop music."
- This is considered **relevant** (implicit YES with detail).
- Do not mark as irrelevant due to missing explicit "Yes".
- The `correct` example must be in English, e.g., "Yes, I like pop music!"
```

### Tunable Parameters
```go
model: "openai/gpt-4o-mini"  // LLM model (configurable in YAML)
temperature: 0.3              // Consistency (0.0-1.0, lower = more consistent)
maxTokens: 500                // Response length (longer for detailed feedback)
language: "Vietnamese"        // Target language for feedback translation
stream: false                 // Direct response, not streaming
```

### Language Support
The agent supports multi-language feedback:
- **Short description**: Translated to target language
- **Long description**: Translated to target language with `<b>...</b>` highlights and `<err>...</err>` error tags preserved
- **Example response (correct field)**: Always in English, shows how to properly respond to the AI's message (e.g., answers questions, replies to statements)
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
- **Reason:** Guarantees valid, parseable JSON responses with consistent structure
- **vs Streaming:** EvaluateAgent needs complete validated data, streaming is unnecessary

### Prompt Templates
✅ **Externalized to YAML** - All prompts, guidelines, and criteria stored in `prompts/_evaluate_prompt.yaml` for easy editing.

**Benefits:**
- No code changes needed to update evaluation criteria
- Easy to maintain and version control
- Consistent with other agent configurations
- Support for template variables: `{user_message}`, `{ai_message}`, `{topic}`, `{level}`, `{language}`

## Integration with Existing System

### Implementation Status

✅ **Completed Components:**
1. ✅ EvaluateAgent implementation with structured outputs
2. ✅ YAML configuration system (`_evaluate_prompt.yaml`)
3. ✅ Level-specific evaluation criteria (6 levels)
4. ✅ Multi-language feedback support
5. ✅ JSON schema validation with OpenRouter
6. ✅ Config loading in utils/config.go
7. ✅ Highlight support with `<b>tags</b>`
8. ✅ Three evaluation levels (excellent/good/needs_improvement)
9. ✅ Terminal display formatting
10. ✅ Registration in AgentManager
11. ✅ Integration in ChatbotOrchestrator (CLI)
    - Evaluates user messages before AI response
    - Shows feedback with emoji indicators
    - Displays after each user input (if AI context exists)

⏳ **Pending Integrations:**
1. ⏳ Integration in ChatbotWeb (web interface)
2. ⏳ Frontend UI components for web display

### Current Implementation

**AgentManager Registration (✅ Completed):**
```go
func (m *AgentManager) RegisterAgents(level models.ConversationLevel, topic string, language string) {
    conversationAgent := NewConversationAgent(m.apiClient, level, topic)
    suggestionAgent := NewSuggestionAgent(m.apiClient, level, topic, language)
    evaluateAgent := NewEvaluateAgent(m.apiClient, level, topic, language)

    m.agents[conversationAgent.Name()] = conversationAgent
    m.agents[suggestionAgent.Name()] = suggestionAgent
    m.agents[evaluateAgent.Name()] = evaluateAgent
}
```

**ChatbotOrchestrator Integration (✅ Completed):**
```go
func (co *ChatbotOrchestrator) processUserMessage(userMessage string) {
    // Get last AI message from conversation history
    conversationAgent := co.manager.agents["ConversationAgent"].(*ConversationAgent)
    history := conversationAgent.GetFullConversationHistory()
    lastAIMessage := "" // Extract from history
    
    // Evaluate user's response
    evaluateAgent, evalExists := co.manager.GetAgent("EvaluateAgent")
    if evalExists && lastAIMessage != "" {
        evaluateJob := models.JobRequest{
            Task:          "evaluate",
            UserMessage:   userMessage,
            LastAIMessage: lastAIMessage,
        }
        evaluateResponse := evaluateAgent.ProcessTask(evaluateJob)
        if evaluateResponse.Success {
            ea := evaluateAgent.(*EvaluateAgent)
            ea.DisplayEvaluation(evaluateResponse.Result)
        }
    }
    
    // Continue with conversation and suggestions...
}
```

**Flow Order:**
1. User enters message
2. **Evaluate user's response** (if AI context exists)
3. Get AI response from ConversationAgent
4. Show vocabulary suggestions from SuggestionAgent

**Next Steps:**
- ⏳ Add to ChatbotWeb (web interface)
- ⏳ Frontend UI with HTML highlight support for `<b>tags</b>`

## Comparison with SuggestionAgent

### Similarities
- Uses OpenRouter Structured Outputs
- YAML-based configuration
- Level-adaptive behavior
- Multi-language support
- Structured JSON responses
- Similar architecture and patterns

### Differences
- **Temperature**: 0.3 (EvaluateAgent) vs 0.7 (SuggestionAgent)
  - Evaluation needs consistency, suggestions need creativity
  
- **MaxTokens**: 500 (EvaluateAgent) vs 150 (SuggestionAgent)
  - Detailed feedback needs more space, suggestions are brief
  
- **Response Fields**: Status/descriptions/correction vs leading sentence/vocab options
  
- **Purpose**: Evaluate existing response vs guide future response
  
- **Tone**: Analytical + encouraging vs creative + helpful
  
- **Input**: User's message + AI's context vs AI's message only

## Example Usage Flow

```go
// User sends message
userMessage := "I am playing soccer for 5 years"
aiContext := "How long have you been playing soccer?"

// Evaluate user's response
evaluateTask := models.JobRequest{
    Task:          "evaluate",
    UserMessage:   userMessage,
    LastAIMessage: aiContext,
}

evaluateResponse := evaluateAgent.ProcessTask(evaluateTask)
evaluateAgent.DisplayEvaluation(evaluateResponse.Result)

// Output:
// 📊 Evaluation:
// ────────────────────────────────────────
// 👍 Status: GOOD
// 
// 💬 Câu trả lời tốt! Ý tưởng của bạn rõ ràng.
// 
// 📖 Detailed Feedback:
// Bạn cần sửa thì của động từ. Khi nói về hành động bắt đầu 
// từ quá khứ và tiếp tục đến hiện tại, dùng <b>present perfect 
// continuous</b>: <b>"have been playing"</b> thay vì <b>"am playing"</b>.
// 
// ✅ Corrected: I have been playing soccer for 5 years.
// ────────────────────────────────────────
```

