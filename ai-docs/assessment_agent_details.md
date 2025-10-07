# AssessmentAgent Implementation Details

## Overview

The AssessmentAgent is a specialized agent that analyzes conversation history to provide comprehensive proficiency assessments and learning tips. It evaluates learner performance across multiple interactions to determine their current CEFR level and provides actionable improvement suggestions.

## Key Features

### ✅ Implemented Features

- **Proficiency Level Assessment**: Determines current CEFR level (A1-C2)
- **General Skills Evaluation**: Describes what learners can do at their level
- **Learning Tips Generation**: Provides specific, actionable tips for improvement
- **Conversation History Analysis**: Analyzes patterns across multiple interactions
- **Structured Output**: Uses OpenRouter Structured Outputs for consistent responses
- **Evaluation Integration**: Factors in previous evaluation feedback

### ⏳ Pending Features

- Integration with web interface
- Export assessment results
- Historical assessment tracking
- Comparative analysis across sessions

## Architecture

### Core Components

```go
type AssessmentAgent struct {
    name        string
    client      client.Client
    language    string
    model       string
    temperature float64
    maxTokens   int
    config      *utils.AssessmentPromptConfig
}
```

### Response Structure

```go
type AssessmentResponse struct {
    Level                 string   `json:"level"`                  // CEFR level (A1-C2)
    GeneralSkills         string   `json:"general_skills"`         // What learner can do (max 10 words)
    GrammarTips           []string `json:"grammar_tips"`           // List of tagged strings: <t>title</t><d>description</d>
    VocabularyTips        []string `json:"vocabulary_tips"`        // List of tagged strings: <t>title</t><d>description</d>
    FluencySuggestions    []string `json:"fluency_suggestions"`    // List of tagged strings: <t>title</t><d>description</d><s>phrase</s>
    VocabularySuggestions []string `json:"vocabulary_suggestions"` // List of tagged strings: <t>title</t><d>description</d><v>vocab</v>
}

type TipObject struct {
    Title       string `json:"title"`       // Short description of which tense/grammar/vocabulary to use
    Description string `json:"description"` // Detailed explanation of usage with examples
}

type FluencySuggestion struct {
    Title       string   `json:"title"`       // Short description of fluency improvement area
    Description string   `json:"description"` // Explanation of what phrases to learn and why
    Phrases     []string `json:"phrases"`     // List of useful phrases
}

type VocabSuggestion struct {
    Title       string   `json:"title"`       // Short description of vocabulary improvement area
    Description string   `json:"description"` // Explanation of what vocabulary to learn and why
    Vocab       []string `json:"vocab"`       // List of useful vocabulary words
}
```

## Configuration

### YAML Configuration (`_assessment_prompt.yaml`)

```yaml
assessment_agent:
  llm:
    model: "openai/gpt-4o-mini"
    temperature: 0.2
    max_tokens: 800

  base_prompt: |
    You are an expert English language assessment specialist...
    
  user_prompt_template: |
    Analyze this conversation history and provide assessment...
    
  level_descriptions:
    A1:
      name: "Beginner"
      description: "Can understand and use familiar everyday expressions..."
      skills:
        - "Introduce themselves and others"
        - "Ask and answer basic personal questions"
        # ... more skills
      grammar_focus: "Present simple, past simple, basic sentence structure"
      vocabulary_focus: "Everyday objects, family, basic activities"
      expression_focus: "Simple questions and answers, basic social interactions"
    # ... other levels (A2, B1, B2, C1, C2)
```

## Implementation Details

### History Filtering

The agent filters conversation history to include only relevant messages:

```go
func (aa *AssessmentAgent) filterHistoryForAssessment(history []models.Message) []models.Message {
    var filtered []models.Message
    
    for _, msg := range history {
        if msg.Role == models.MessageRoleAssistant || msg.Role == models.MessageRoleUser {
            filteredMsg := models.Message{
                Index:   msg.Index,
                Role:    msg.Role,
                Content: msg.Content,
            }
            
            // Include evaluation data for user messages
            if msg.Role == models.MessageRoleUser && msg.Evaluation != nil {
                filteredMsg.Evaluation = msg.Evaluation
            }
            
            filtered = append(filtered, filteredMsg)
        }
    }
    
    return filtered
}
```

### Assessment Process

1. **History Retrieval**: Gets conversation history from ConversationHistoryManager
2. **History Filtering**: Removes suggestions, keeps AI messages, user messages, and evaluations
3. **Analysis**: Uses LLM to analyze patterns and determine proficiency level
4. **Response Generation**: Creates structured assessment with level and tips

### JSON Schema Validation

The agent uses OpenRouter Structured Outputs to ensure consistent, type-safe responses. The JSON schema is properly nested within the `json_schema` object as required by the OpenRouter API.

```go
func (aa *AssessmentAgent) buildResponseFormat() *models.ResponseFormat {
    schema := map[string]any{
        "type": "object",
        "properties": map[string]any{
            "level": map[string]any{
                "type":        "string",
                "enum":        []string{"A1", "A2", "B1", "B2", "C1", "C2"},
                "description": "The learner's current CEFR proficiency level",
            },
            "general_skills": map[string]any{
                "type":        "string",
                "description": "Description of what the learner can do at their current level (in target language, maximum 10 words)",
            },
            "grammar_tips": map[string]any{
                "type":        "array",
                "items":       map[string]any{"type": "string"},
                "description": "List of 2-4 grammar improvement tips, each formatted as: <t>title</t><d>description</d>",
            },
            "vocabulary_tips": map[string]any{
                "type":        "array",
                "items":       map[string]any{"type": "string"},
                "description": "List of 2-4 vocabulary expansion tips, each formatted as: <t>title</t><d>description</d>",
            },
            "fluency_suggestions": map[string]any{
                "type":        "array",
                "items":       map[string]any{"type": "string"},
                "description": "List of 2-5 fluency improvement suggestions, each formatted as: <t>title</t><d>description</d><s>phrase1</s><s>phrase2</s> (multiple tags supported)",
            },
            "vocabulary_suggestions": map[string]any{
                "type":        "array",
                "items":       map[string]any{"type": "string"},
                "description": "List of 2-5 vocabulary improvement suggestions, each formatted as: <t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v> (minimum 4 vocab words required, multiple tags supported)",
            },
        },
        "required":             []string{"level", "general_skills", "grammar_tips", "vocabulary_tips", "fluency_suggestions", "vocabulary_suggestions"},
        "additionalProperties": false,
    }
    // ... return ResponseFormat
}
```

**Key Features:**
- Uses `strict: true` to ensure exact schema compliance
- Properly nested schema structure as required by OpenRouter API
- Comprehensive validation for all response fields
- Support for tagged string format in grammar_tip and vocabulary_tip arrays

### Tagged String Parsing

The agent includes a helper function to parse tagged strings:

```go
func (aa *AssessmentAgent) parseTaggedString(taggedString string) []TipObject {
    // Extracts title-description pairs from <t></t><d></d> tags
    // Returns a list of TipObject structs with individual title and description
    // Each pair becomes a separate TipObject in the returned slice
}

func (aa *AssessmentAgent) parseFluencySuggestion(taggedString string) []FluencySuggestion {
    // Extracts title-description-phrases groups from <t></t><d></d><s></s> tags
    // Returns a list of FluencySuggestion structs with title, description, and phrases
    // Each group becomes a separate FluencySuggestion in the returned slice
}

func (aa *AssessmentAgent) parseVocabSuggestion(taggedString string) []VocabSuggestion {
    // Extracts title-description-vocab groups from <t></t><d></d><v></v> tags
    // Returns a list of VocabSuggestion structs with title, description, and vocab
    // Each group becomes a separate VocabSuggestion in the returned slice
}
```

**Formats**: Different tip types use different tag patterns:

**Grammar/Vocabulary Tips**: `<t>title</t><d>description</d>`
- `<t>title</t>`: Short description of which tense/grammar/vocabulary to use in which context
- `<d>description</d>`: Detailed explanation of usage with examples

**Fluency Suggestions**: `<t>title</t><d>description</d><s>phrase1</s><s>phrase2</s>`
- `<t>title</t>`: Short description of fluency improvement area
- `<d>description</d>`: Explanation of what phrases to learn and why
- `<s>phrase</s>`: Useful phrases for natural conversation

**Vocabulary Suggestions**: `<t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v>`
- `<t>title</t>`: Short description of vocabulary improvement area
- `<d>description</d>`: Explanation of what vocabulary to learn and why
- `<v>vocab</v>`: Useful vocabulary words (minimum 4 words required)

**Examples**:
```
// Grammar tip example:
"<t>Present Continuous cho hành động đang diễn ra</t><d>Luyện tập sử dụng \"I am playing\" thay vì \"I play\" khi nói về hành động đang diễn ra. Ví dụ: \"I am playing football now\" thay vì \"I play football now\"</d>"

// Fluency suggestion example:
"<t>Expressing opinions</t><d>Học các cụm từ để bày tỏ ý kiến một cách tự nhiên</d><s>I think that</s><s>In my opinion</s><s>I believe</s>"

// Vocabulary suggestion example:
"<t>Sports vocabulary</t><d>Mở rộng từ vựng về thể thao để nói chuyện tự nhiên hơn</d><v>tournament</v><v>championship</v><v>training</v><v>competition</v>"
```

## Integration Points

### AgentManager Registration

```go
func (m *AgentManager) RegisterAgents(level models.ConversationLevel, topic string, language string) {
    // ... other agents
    assessmentAgent := agents.NewAssessmentAgent(m.apiClient, language)
    m.agents[assessmentAgent.Name()] = assessmentAgent
}
```

### Special Metadata Handling

```go
func (m *AgentManager) ProcessJob(job models.JobRequest) *models.JobResponse {
    // ... agent selection
    
    // Special handling for AssessmentAgent - it needs the history manager
    if agent.Name() == "AssessmentAgent" {
        job.Metadata = m.historyManager
    }
    
    return agent.ProcessTask(job)
}
```

## Usage Examples

### CLI Usage

```bash
# Trigger assessment
/assess
```

### Programmatic Usage

```go
// Get assessment agent
assessmentAgent := manager.GetAssessmentAgent()

// Create assessment task
task := models.JobRequest{
    Task:        "assess proficiency level",
    UserMessage: "",
    LastAIMessage: "",
    Metadata:    historyManager,
}

// Process assessment
response := assessmentAgent.ProcessTask(task)
if response.Success {
    assessmentAgent.DisplayAssessment(response.Result)
}
```

## Display Format

The agent provides formatted output with emojis and clear sections:

```
📊 Proficiency Assessment:
────────────────────────────────────────
🌿 Level: A2

🎯 General Skills:
Bạn có thể nói cơ bản về chủ đề bóng đá

📚 Grammar Tips:
• Present Continuous cho hành động đang diễn ra
  Luyện tập sử dụng "I am playing" thay vì "I play" khi nói về hành động đang diễn ra. Ví dụ: "I am playing football now" thay vì "I play football now"
• Past Simple cho hành động đã xảy ra
  Sử dụng động từ quá khứ đơn để nói về những gì đã xảy ra. Ví dụ: "I played football yesterday" hoặc "We watched the match last week"

📖 Vocabulary Tips:
• Từ vựng thể thao cơ bản
  Học thêm từ vựng về các môn thể thao khác như "tennis", "basketball", "swimming". Ví dụ: "I like playing tennis" hoặc "Swimming is good exercise"
• Động từ thể thao
  Luyện tập các động từ thể thao như "kick", "throw", "catch", "run". Ví dụ: "I kick the ball" hoặc "He throws the ball to me"

🗣️ Fluency Suggestions:
• Expressing opinions
  Học các cụm từ để bày tỏ ý kiến một cách tự nhiên
  Phrases: I think that, In my opinion, I believe

📚 Vocabulary Suggestions:
• Sports vocabulary
  Mở rộng từ vựng về thể thao để nói chuyện tự nhiên hơn
  Vocabulary: tournament, championship, training

**Note**: The tips are stored as tagged strings in various formats and parsed into individual structs for display. Each title-description pair becomes a separate object in the list.
────────────────────────────────────────
```

## CEFR Level Descriptions

### A1 - Beginner
- **Skills**: Basic introductions, simple questions, present/past tenses
- **Grammar Focus**: Present simple, past simple, basic sentence structure
- **Vocabulary Focus**: Everyday objects, family, basic activities
- **Expression Focus**: Simple questions and answers, basic social interactions

### A2 - Elementary
- **Skills**: Describe background, express needs, use multiple tenses
- **Grammar Focus**: Present continuous, future forms, basic conditionals
- **Vocabulary Focus**: Work, travel, hobbies, personal experiences
- **Expression Focus**: Expressing opinions, making plans, describing experiences

### B1 - Intermediate
- **Skills**: Deal with travel situations, produce connected text, describe experiences
- **Grammar Focus**: Present perfect, past continuous, conditionals, modal verbs
- **Vocabulary Focus**: Abstract concepts, opinions, complex topics
- **Expression Focus**: Expressing agreement/disagreement, giving detailed explanations

### B2 - Upper Intermediate
- **Skills**: Interact with native speakers, produce detailed text, explain viewpoints
- **Grammar Focus**: Advanced conditionals, passive voice, complex sentence structures
- **Vocabulary Focus**: Professional topics, nuanced expressions, idiomatic language
- **Expression Focus**: Formal and informal registers, sophisticated argumentation

### C1 - Advanced
- **Skills**: Understand demanding texts, express ideas fluently, use language flexibly
- **Grammar Focus**: Advanced grammar structures, stylistic variation, register awareness
- **Vocabulary Focus**: Specialized vocabulary, subtle distinctions, cultural references
- **Expression Focus**: Sophisticated communication, cultural awareness, nuanced expression

### C2 - Proficient
- **Skills**: Understand everything, express spontaneously, differentiate subtle meanings
- **Grammar Focus**: Native-like accuracy, stylistic mastery, register perfection
- **Vocabulary Focus**: Extensive vocabulary, cultural idioms, professional terminology
- **Expression Focus**: Native-like fluency, cultural competence, professional communication

## Assessment Criteria

### Grammar Analysis
- Verb tense accuracy and consistency
- Sentence structure complexity
- Word order and syntax
- Use of articles, prepositions, and connectors
- Error patterns and frequency

### Vocabulary Analysis
- Range of vocabulary used
- Appropriateness of word choice
- Use of collocations and phrasal verbs
- Ability to express abstract concepts
- Vocabulary accuracy and precision

### Expression Analysis
- Fluency and naturalness
- Ability to express opinions and ideas
- Use of appropriate register
- Communication strategies
- Cultural awareness and appropriateness

## Key Principles

- **Performance-Based**: Base assessment on actual performance, not potential
- **Consistency Focus**: Consider patterns across multiple interactions
- **Evaluation Integration**: Factor in previous evaluation feedback
- **Encouraging Tone**: Be encouraging while being realistic about level
- **Actionable Tips**: Provide specific, actionable improvement suggestions
- **Example-Based**: Reference concrete examples from the conversation
- **Priority Focus**: Focus on the most important areas for improvement
- **Context Awareness**: Consider the learner's communication goals and context

## Error Handling

- **No History**: Returns error if no conversation history available
- **No Relevant Messages**: Returns error if no assessable messages found
- **LLM Failure**: Returns error if assessment generation fails
- **Invalid Metadata**: Returns error if ConversationHistoryManager not provided

## Future Enhancements

1. **Web Integration**: Add assessment endpoint to web interface
2. **Export Functionality**: Allow exporting assessment results
3. **Historical Tracking**: Track assessment progress over time
4. **Comparative Analysis**: Compare assessments across different sessions
5. **Custom Tips**: Generate tips based on specific learning goals
6. **Progress Metrics**: Provide quantitative progress indicators
7. **Learning Paths**: Suggest specific learning paths based on assessment
8. **Skill Breakdown**: Provide detailed breakdown by skill area (listening, speaking, reading, writing)

## Dependencies

- **OpenRouter Client**: For LLM communication with structured outputs
- **ConversationHistoryManager**: For accessing conversation history
- **Utils Config**: For loading assessment prompt configuration
- **Models**: For message structures and response formats

## Testing Considerations

- Test with various conversation lengths
- Test with different proficiency levels
- Test error handling scenarios
- Test JSON schema validation
- Test display formatting
- Test integration with other agents
