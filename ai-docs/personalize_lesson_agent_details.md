# PersonalizeLessonAgent - Detailed Implementation Guide

## Overview

The PersonalizeLessonAgent is a specialized agent that creates personalized lesson details based on user preferences. When users input their desired topic, proficiency level, and native language, the agent generates an engaging lesson detail with an emoji, title, description, and 4 essential vocabulary words tailored to their learning needs.

## Key Features

### ✅ Implemented Features

- **Personalized Lesson Detail Generation**: Creates custom lesson details based on user input
- **Emoji Integration**: Generates relevant emojis that represent the learning topic
- **Engaging Titles**: Creates attractive, motivating lesson titles
- **Compelling Descriptions**: Writes descriptions that explain learning benefits
- **Essential Vocabulary Creation**: Generates 4 vocabulary words with meanings and example sentences
- **Level-Adaptive Content**: Adjusts content complexity based on proficiency level
- **Multi-Language Support**: Supports various native languages for personalized experience
- **OpenRouter Structured Outputs**: Uses JSON schema validation for consistent responses
- **YAML Configuration**: Externalized prompts and settings for easy customization

### ⏳ Pending Features

- Integration with web interface
- Lesson content generation (detail lists, exercises)
- Progress tracking for personalized lessons
- Lesson difficulty adjustment based on user performance

## Architecture

### Core Components

```go
type PersonalizeLessonAgent struct {
    name        string
    client      client.Client
    model       string
    temperature float64
    maxTokens   int
    config      *utils.PersonalizeLessonPromptConfig
}
```

#### Constants

```go
const (
    agentNamePersonalizeLesson           = "PersonalizeLessonAgent"
    defaultModelPersonalizeLesson        = "openai/gpt-4o-mini"
    defaultTemperaturePersonalizeLesson  = 0.8
    defaultMaxTokensPersonalizeLesson    = 1000
    schemaNamePersonalizeLessonResponse  = "personalize_lesson_response"
)
```

These constants centralize defaults and identifiers used by the agent.

### Response Structure

```go
type PersonalizeVocabItem struct {
    Vocab            string `json:"vocab"`             // English vocabulary word
    Meaning          string `json:"meaning"`           // Meaning in native language
    Sentence         string `json:"sentence"`          // Example sentence with vocab highlighted in <b>...</b>
    SentenceMeaning  string `json:"sentence_meaning"`  // Translation of the sentence in native language
}

type PersonalizeLessonResponse struct {
    Emoji        string               `json:"emoji"`        // Relevant emoji for the topic
    Title        string               `json:"title"`        // Engaging lesson title
    Description  string               `json:"description"`  // Motivating lesson description
    Vocabulary   []PersonalizeVocabItem `json:"vocabulary"` // 4 essential vocabulary items
}
```

## Implementation Details

### Agent Initialization

```go
func NewPersonalizeLessonAgent(client client.Client) *PersonalizeLessonAgent
```

**Parameters:**
- `client`: OpenRouter client for API communication

**Default Settings:**
- Model: `openai/gpt-4o-mini`
- Temperature: `0.8` (creative responses)
- Max Tokens: `1000`

**Note:** All lesson detail parameters (topic, level, language) are passed through JobRequest metadata, making the agent more flexible and reusable. The agent no longer stores these parameters internally.

### Task Processing

The agent processes tasks through the `ProcessTask()` method:

```go
func (pla *PersonalizeLessonAgent) ProcessTask(task models.JobRequest) *models.JobResponse
```

**Task Recognition:**
The agent can handle tasks containing:
- "personalize"
- "lesson"
- "detail"
- "create"
- "vocabulary"
- "vocab"

**Metadata Processing:**
The agent extracts lesson detail parameters from the JobRequest metadata:
- `topic`: The learning topic (e.g., "music", "sports", "travel")
- `level`: The proficiency level (beginner, elementary, intermediate, etc.)
- `language`: The user's native language for personalization

If metadata is not provided, the agent uses sensible default values (topic: "general", level: "intermediate", language: "English").

### Metadata Extraction

The agent includes a robust metadata extraction system:

```go
func (pla *PersonalizeLessonAgent) extractMetadata(metadata any) (string, models.ConversationLevel, string)
```

**Extraction Logic:**
1. **Default Values**: Uses sensible defaults (topic: "general", level: "intermediate", language: "English")
2. **Type Safety**: Validates metadata is a `map[string]any`
3. **Field Validation**: 
   - Validates topic is non-empty string
   - Validates level using `models.IsValidConversationLevel()`
   - Validates language is non-empty string
4. **Graceful Fallback**: Returns default values if metadata is invalid

**Metadata Structure:**
```go
metadata := map[string]any{
    "topic":    "sports",           // Learning topic
    "level":    "beginner",         // Proficiency level
    "language": "Vietnamese",       // Native language
}
```

### Prompt System

#### Base Prompt
The agent uses a careful vocabulary lesson designer persona that:
- Generates clear, concise vocabulary lessons based on user preferences
- Chooses ONE most relevant emoji that clearly represents the topic (selective and precise)
- Creates short, clear titles in English (under 6 words, easy to understand)
- Writes concise descriptions in the learner's native language (under 2 sentences, focus on practical benefits)
- Creates 4 essential vocabulary words related to the topic and appropriate for the learner's level
- For each vocabulary word: chooses English words essential for understanding the topic, formats vocabulary as "word (type)" where type is n. = noun, v. = verb, adj. = adjective, adv. = adverb, provides a very short meaning in native language (2-4 words max), creates English sentences using the word in context, highlights vocabulary between <b>...</b> tags
- Emphasizes careful emoji selection and simplicity for language learners

#### Level-Specific Guidelines

**Beginner Level:**
- Simple, fun, and encouraging approach
- Basic, everyday topics
- Very short titles (3-4 words max)
- Most obvious emoji selection
- Simple, encouraging descriptions (1 sentence)
- Focus on practical, useful vocabulary
- Choose very basic English words (A1 level)
- Use simple sentences with basic grammar
- Provide clear, simple meanings in native language

**Elementary Level:**
- Practical and engaging content
- Common situations focus
- Short, clear titles (4-5 words max)
- Obvious, practical emojis
- Concise descriptions (1-2 sentences)
- Emphasis on practical benefits
- Choose common English words (A2 level)
- Use present tense and simple past tense
- Provide practical, everyday meanings

**Intermediate Level:**
- Interesting and varied topics
- Clear, engaging titles (5-6 words max)
- Relevant, clear emojis
- Focused descriptions (1-2 sentences)
- Highlight practical learning benefits
- Choose intermediate English words (B1 level)
- Use various tenses and complex sentences
- Provide nuanced meanings and context

**Upper Intermediate Level:**
- Sophisticated and challenging content
- Complex topics focus
- Sophisticated but clear titles (5-6 words max)
- Professional, relevant emojis
- Concise, motivating descriptions (1-2 sentences)
- Advanced learning goals emphasis
- Choose advanced English words (B2 level)
- Use complex grammar and sophisticated expressions
- Provide detailed, professional meanings

**Advanced Level:**
- Nuanced and sophisticated approach
- Nuanced topics exploration
- Sophisticated but clear titles (5-6 words max)
- Culturally relevant emojis
- Concise, insightful descriptions (1-2 sentences)
- Native-like expression focus
- Choose sophisticated English words (C1 level)
- Use idiomatic expressions and cultural references
- Provide nuanced, culturally-aware meanings

**Fluent Level:**
- Authentic and sophisticated content
- Authentic, native-level topics
- Sophisticated but clear titles (5-6 words max)
- Intellectually relevant emojis
- Concise, insightful descriptions (1-2 sentences)
- Cultural fluency and depth emphasis
- Choose native-level English words (C2 level)
- Use sophisticated, academic, or literary expressions
- Provide deep, contextual meanings with cultural nuances

### JSON Schema Validation

The agent uses OpenRouter's structured outputs with strict JSON schema validation:

```json
{
  "type": "object",
  "properties": {
    "emoji": {
      "type": "string",
      "description": "ONE clear emoji that best represents the topic (choose the most obvious one)"
    },
    "title": {
      "type": "string", 
      "description": "A short, clear title in English (under 6 words, easy to understand)"
    },
    "description": {
      "type": "string",
      "description": "A concise description in the learner's native language (under 2 sentences, focus on practical benefits)"
    },
    "vocabulary": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "vocab": {
            "type": "string",
            "description": "English vocabulary word formatted as 'word (type)' where type is n. = noun, v. = verb, adj. = adjective, adv. = adverb"
          },
          "meaning": {
            "type": "string",
          "description": "Very short meaning in the learner's native language (2-4 words max)"
          },
          "sentence": {
            "type": "string",
          "description": "English example sentence using the word in context, with the word highlighted between <b>...</b> tags"
          },
          "sentence_meaning": {
            "type": "string",
            "description": "Translation of the example sentence in the learner's native language"
          }
        },
        "required": ["vocab", "meaning", "sentence", "sentence_meaning"],
        "additionalProperties": false
      },
      "minItems": 4,
      "maxItems": 4,
      "description": "Exactly 4 essential vocabulary words related to the topic and appropriate for the learner's level"
    }
  },
  "required": ["emoji", "title", "description", "vocabulary"],
  "additionalProperties": false
}
```

## Configuration

### YAML Configuration File

The agent configuration is stored in `prompts/_personalize_vocab_prompt.yaml`:

```yaml
personalize_vocab_agent:
  llm:
    model: "openai/gpt-4o-mini"
    temperature: 0.8
    max_tokens: 200

  base_prompt: |
    You are a careful vocabulary lesson designer that creates personalized English learning experiences for non-native speakers.
    
    Your role is to generate clear, concise vocabulary lessons that help learners understand English words:
    - Choose ONE most relevant emoji that clearly represents the topic (be selective and precise)
    - Create a short, clear title in English (under 6 words, easy to understand)
    - Write a concise description in their native language (under 2 sentences, focus on practical benefits)
    
    Be careful with emoji selection - choose the most obvious and universally understood emoji for the topic.
    Keep everything simple, clear, and practical for language learners.
    
  user_prompt_template: |
    Create a personalized English vocabulary lesson for a {nativeLanguage} speaker:
    
    Topic: {topic}
    English Level: {level}
    Native Language: {nativeLanguage}
    
    Generate:
    1. ONE clear emoji that best represents this topic (choose the most obvious one)
    2. A short, clear title in English (under 6 words, easy to understand)
    3. A concise description in {nativeLanguage} (under 2 sentences, focus on practical benefits)
    
    Be precise with emoji selection and keep everything simple and practical.
    
  level_guidelines:
    beginner:
      name: "Beginner"
      description: "Simple, fun, and encouraging"
      guidelines: [...]
      example_emoji: "🏠"
      example_title: "My Home"
      example_description: "Học từ vựng cơ bản về nhà cửa và gia đình."
      
  key_principles:
    - "Be careful and precise with emoji selection - choose the most obvious one"
    - "Keep titles short, clear, and easy to understand (under 6 words)"
    - "Write concise descriptions (under 2 sentences) in the learner's native language"
    - "Focus on practical benefits and real-world application"
    - "Make everything simple and clear for language learners"
    - "Choose universally understood emojis that clearly represent the topic"
```

### Configuration Loading

```go
func LoadPersonalizeVocabConfig() (*PersonalizeVocabPromptConfig, error)
```

The configuration is cached in memory for performance and loaded on first use.

## Integration Points

### PersonalizeManager Registration

The agent is automatically registered in the PersonalizeManager:

```go
func (pm *PersonalizeManager) RegisterAgents() {
    personalizeLessonAgent := agents.NewPersonalizeLessonAgent(pm.client)
    pm.agents[personalizeLessonAgent.Name()] = personalizeLessonAgent
}
```

### Agent Retrieval

```go
func (pm *PersonalizeManager) GetAgent(name string) (models.Agent, bool)
```

The PersonalizeManager handles task routing and delegates to the PersonalizeLessonAgent when appropriate.

## Usage Examples

### Basic Usage

```go
// Initialize PersonalizeManager
manager := managers.NewPersonalizeManager(client)

// Process task with metadata
task := models.JobRequest{
    Task: "create personalized lesson detail",
    Metadata: map[string]any{
        "topic":    "sports",
        "level":    "beginner",
        "language": "Vietnamese",
    },
}

response := manager.ProcessTask(task)
if response.Success {
    // Display the personalized lesson
    fmt.Println(response.Result)
}
```

### Expected Output

```
🎯 Personalized Lesson Detail:
────────────────────────────────────────
🎵 Music Adventures

📝 Học từ vựng âm nhạc để mô tả bài hát và nhạc cụ một cách tự nhiên.

📚 Essential Vocabulary:
1. <b>melody</b> - giai điệu
   The beautiful <b>melody</b> of the song made everyone smile.

2. <b>rhythm</b> - nhịp điệu
   The drummer kept the <b>rhythm</b> steady throughout the performance.

3. <b>harmony</b> - hòa âm
   The singers created perfect <b>harmony</b> with their voices.

4. <b>instrument</b> - nhạc cụ
   She learned to play the piano, her favorite <b>instrument</b>.

────────────────────────────────────────
```

## Display Methods

### DisplayPersonalizedLesson()

```go
func (pla *PersonalizeLessonAgent) DisplayPersonalizedLesson(jsonResponse string)
```

Formats and displays the personalized lesson detail with:
- Emoji and title on the same line
- Description with proper formatting
- Vocabulary list with numbered items
- Each vocabulary item shows: word, meaning, and example sentence
- Visual separators for clarity

## Error Handling

The agent includes comprehensive error handling:

- **Configuration Loading**: Graceful fallback to default settings
- **API Communication**: Error logging and empty response handling
- **JSON Parsing**: Clean JSON extraction and validation
- **Level Validation**: Automatic fallback to intermediate level

## Performance Considerations

- **Configuration Caching**: YAML configs are cached in memory
- **Structured Outputs**: Eliminates retry logic and parsing errors
- **Efficient Prompt Building**: Template-based prompt construction
- **Memory Management**: Proper cleanup of cached configurations

## Future Enhancements

### Planned Features

1. **Lesson Content Generation**
   - Vocabulary word lists
   - Example sentences
   - Practice exercises
   - Audio pronunciation guides

2. **Web Interface Integration**
   - REST API endpoints
   - Interactive lesson creation
   - Visual lesson previews
   - User preference storage

3. **Advanced Personalization**
   - Learning style adaptation
   - Progress-based difficulty adjustment
   - Topic recommendation engine
   - Cultural context integration

4. **Analytics and Tracking**
   - Lesson completion rates
   - User engagement metrics
   - Learning effectiveness analysis
   - Personalized recommendations

## Testing

### Unit Tests

```go
func TestPersonalizeLessonAgent_CanHandle(t *testing.T) {
    agent := NewPersonalizeLessonAgent(mockClient)
    
    assert.True(t, agent.CanHandle("create personalized lesson detail"))
    assert.True(t, agent.CanHandle("personalize lesson"))
    assert.False(t, agent.CanHandle("unrelated task"))
}

func TestPersonalizeManager_CanHandle(t *testing.T) {
    manager := NewPersonalizeManager(mockClient)
    
    assert.True(t, manager.CanHandle("create personalized lesson detail"))
    assert.True(t, manager.CanHandle("personalize lesson"))
    assert.False(t, manager.CanHandle("unrelated task"))
}
```

### Integration Tests

```go
func TestPersonalizeLessonAgent_ProcessTask(t *testing.T) {
    agent := NewPersonalizeLessonAgent(realClient)
    
    task := models.JobRequest{
        Task: "create personalized lesson detail",
        Metadata: map[string]any{
            "topic":    "food",
            "level":    "beginner",
            "language": "Spanish",
        },
    }
    
    response := agent.ProcessTask(task)
    assert.True(t, response.Success)
    assert.Contains(t, response.Result, "emoji")
    assert.Contains(t, response.Result, "title")
    assert.Contains(t, response.Result, "description")
}

func TestPersonalizeManager_ProcessTask(t *testing.T) {
    manager := NewPersonalizeManager(realClient)
    
    task := models.JobRequest{
        Task: "create personalized lesson detail",
        Metadata: map[string]any{
            "topic":    "sports",
            "level":    "intermediate",
            "language": "French",
        },
    }
    
    response := manager.ProcessTask(task)
    assert.True(t, response.Success)
    assert.Equal(t, "PersonalizeManager", response.AgentName)
}
```

## Troubleshooting

### Common Issues

1. **Configuration Not Found**
   - Ensure `_personalize_lesson_prompt.yaml` exists in prompts directory
   - Check file permissions and YAML syntax

2. **API Communication Errors**
   - Verify OpenRouter API key is valid
   - Check network connectivity
   - Review API rate limits

3. **JSON Parsing Errors**
   - Ensure structured outputs are enabled
   - Check JSON schema validation
   - Verify response format compliance

### Debug Mode

Enable debug logging to trace agent execution:

```go
utils.PrintInfo(fmt.Sprintf("PersonalizeLessonAgent processing task: %s", task.Task))
utils.PrintInfo(fmt.Sprintf("Using model: %s, temperature: %f", pla.model, pla.temperature))
```

## Related Documentation

- [Agents Overview](agents_overview.md) - General agent architecture including PersonalizeManager
- [SuggestionAgent Details](suggestion_agent_details.md) - Similar structured output implementation
- [ConversationAgent Details](conversation_agent_details.md) - Base agent patterns
- [ChatbotWeb Details](chatbot_web_details.md) - Web interface integration

## Version History

- **v1.0.0** - Initial implementation with basic lesson generation
- **v1.1.0** - Added level-specific guidelines and examples
- **v1.2.0** - Enhanced error handling and configuration caching
- **v1.3.0** - Improved JSON schema validation and response formatting
- **v1.4.0** - Refactored to use metadata-based parameter passing, removed internal state storage
- **v1.5.0** - Added PersonalizeManager for better task coordination and agent management
- **v1.6.0** - Enhanced prompt configuration for careful emoji selection, concise titles (under 6 words), and short descriptions (under 2 sentences) for better clarity and simplicity
- **v1.7.0** - Renamed from PersonalizeVocabularyAgent to PersonalizeLessonAgent for better clarity and updated all related files and documentation
- **v1.8.0** - Added vocabulary creation feature with 4 essential vocabulary words, meanings in native language, and example sentences with highlighted vocabulary words
