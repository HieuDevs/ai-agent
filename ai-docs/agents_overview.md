# Agents Architecture Overview

## Agent System Components

The agents package contains a modular AI agent system for an English conversation learning application. The system is designed to help users practice English at different proficiency levels.

### Core Components

#### 1. PersonalizeManager (`managers/personalize_manager.go`)
Specialized manager for personalization-related agents and vocabulary lesson creation.

**Responsibilities:**
- Register and manage personalization agents
- Route personalization tasks to appropriate agents
- Coordinate vocabulary lesson creation
- Handle personalize-related job requests
- Manage agent selection for personalization tasks

**Key Methods:**
- `NewPersonalizeManager(client)` - Initialize manager with OpenRouter client
- `RegisterAgents()` - Register PersonalizeVocabularyAgent
- `SelectAgent(task)` - Choose appropriate agent for personalization task
- `ProcessTask(task)` - Execute personalization job with selected agent
- `GetAgent(name)` - Retrieve specific agent by name
- `CanHandle(task)` - Check if manager can handle the task

**Capabilities:**
- personalize_management
- agent_coordination

**Task Recognition:**
The manager can handle tasks containing:
- "personalize"
- "vocab" 
- "lesson"
- "create"

#### 2. ConversationAgent (`agents/conversation_agent.go`)
Main agent handling English conversation practice.

**Responsibilities:**
- Manage conversation flow and context
- Generate level-appropriate responses
- Support streaming responses
- Integrate Vietnamese translations
- Work with ConversationHistoryManager

**Key Features:**
- Support for 6 proficiency levels (beginner to fluent)
- Dynamic prompt loading based on level and topic
- Integration with ConversationHistoryManager service
- LLM settings per level (model, temperature, max_tokens)
- Automatic Vietnamese translation after responses

**Capabilities:**
- english_conversation
- teaching_response
- conversation_starter
- contextual_responses

#### 3. PersonalizeLessonAgent (`agents/personalize_lesson_agent.go`)
Creates personalized lesson details based on user preferences.

**Responsibilities:**
- Generate engaging lesson details
- Create relevant emojis for topics
- Design attractive lesson titles
- Write motivating descriptions
- Adapt content to proficiency levels
- Extract metadata from JobRequest for flexible parameter handling

**Key Features:**
- Personalized lesson detail generation based on topic, level, and native language
- Dynamic parameter extraction from JobRequest metadata
- Emoji integration for visual appeal
- Level-adaptive content guidelines
- OpenRouter structured outputs with JSON schema validation
- YAML configuration for easy customization
- Flexible metadata-based parameter passing
- Graceful fallback to default values

**Capabilities:**
- lesson_detail_creation
- personalized_learning
- lesson_design

**Key Methods:**
- `NewPersonalizeLessonAgent(client)` - Initialize agent with OpenRouter client
- `ProcessTask(task)` - Handle personalization task
- `generatePersonalizedLesson(task)` - Create lesson with metadata extraction
- `extractMetadata(metadata)` - Extract topic, level, and language from metadata
- `buildPersonalizePrompt(level)` - Build level-specific prompts
- `buildUserPrompt(topic, level, language)` - Create user prompt with parameters
- `buildResponseFormat()` - Configure JSON schema for structured outputs
- `DisplayPersonalizedLesson(jsonResponse)` - Format and display lesson results

#### 4. AssessmentAgent (`agents/assessment_agent.go`)
Specialized agent for proficiency assessment and learning tips.

**Responsibilities:**
- Analyze conversation history for proficiency assessment
- Determine current CEFR level (A1-C2)
- Provide learning tips for improvement
- Generate comprehensive skill evaluations

**Key Features:**
- CEFR level assessment (A1, A2, B1, B2, C1, C2)
- General skills evaluation (maximum 10 words in target language)
- Grammar tips with English titles and target language descriptions
- Vocabulary tips with English titles and target language descriptions
- Conversation history analysis
- Structured output with JSON schema validation

**Capabilities:**
- proficiency_assessment
- level_determination
- learning_tips_generation
- conversation_analysis
- level_appropriate_challenge
- Level-specific capabilities (vocabulary, grammar, discussion complexity)

**Key Methods:**
- `ProcessTask(task)` - Handle conversation task
- `generateConversationStarter()` - Create initial message
- `generateConversationalResponse()` - Generate contextual replies
- `SetLevel(level)` - Change conversation difficulty
- `GetLevel()` - Get current conversation level
- `GetTopic()` - Get conversation topic
- `GetClient()` - Get OpenRouter client
- `GetModel()` - Get LLM model name
- `GetTemperature()` - Get temperature setting
- `GetMaxTokens()` - Get max tokens setting

#### 4. ChatbotOrchestrator (`gateway/chatbot_orchestrator.go`)
Terminal-based interactive conversation interface.

**Responsibilities:**
- CLI-based conversation sessions
- User command processing
- Session lifecycle management
- Statistics and history display
- Integration with all agents

**Commands:**
- `quit/exit` - End session
- `help` - Show available commands
- `stats` - Display conversation statistics
- `history` - Show and export conversation history
- `reset` - Clear conversation history
- `level` - Show current level
- `set level` - Change difficulty level

**Features:**
- Interactive prompt-based interface
- Colored console output
- Conversation history export to JSON
- Real-time statistics tracking
- Integration with EvaluateAgent and SuggestionAgent

#### 5. ChatbotWeb (`gateway/chatbot_web.go`)
Web-based conversation interface with full UI.

**Responsibilities:**
- HTTP server for web interface
- RESTful API endpoints
- Server-sent events for streaming
- Session management
- Prompt file management
- Translation services

**Key Features:**
- Session-based architecture with AgentManager per session
- Parallel evaluation and streaming
- On-demand suggestions
- Embedded HTML/CSS/JavaScript frontend
- YAML prompt editor with validation

**API Endpoints:**
- `GET /` - Serve chat HTML interface
- `GET /api/stream` - Stream AI responses (SSE)
- `POST /api/create-session` - Create new session with topic/level
- `GET /api/topics` - List available topics
- `POST /api/suggestions` - Get vocabulary suggestions
- `GET /api/prompts` - List prompt files
- `GET /api/prompt/content` - Get prompt file content
- `POST /api/prompt/save` - Save edited prompt
- `POST /api/prompt/create` - Create new prompt file
- `POST /api/prompt/delete` - Delete prompt file
- `POST /api/translate` - Translate text to Vietnamese

**UI Features:**
- Real-time streaming responses
- Vietnamese translations for AI messages
- Topic and level selection
- Prompt file editor with YAML validation
- Conversation history display
- Responsive design
- Typing indicators
- Evaluation display with emojis
- On-demand suggestion hints

#### 6. SuggestionAgent (`agents/suggestion_agent.go`)
Provides vocabulary suggestions and sentence starters to help learners respond.

**Responsibilities:**
- Generate contextual vocabulary suggestions
- Provide sentence structure guidance
- Offer emoji-enhanced vocabulary options
- Adapt to learner's proficiency level
- Support multi-language instructions

**Key Features:**
- OpenRouter Structured Outputs with JSON schema validation
- YAML-based configuration (`_suggestion_vocab_prompt.yaml`)
- Three vocabulary options per suggestion
- Multi-language support for leading sentences
- Level-adaptive prompts (6 levels)
- Emoji integration for visual enhancement
- Temperature: 0.7 for creativity

**Response Structure:**
- `leading_sentence` - Guide for response structure (translated)
- `vocab_options` - Three options with text and emoji

**Capabilities:**
- vocabulary_suggestion
- response_guidance
- sentence_completion

**Status:** ✅ Implemented and integrated in CLI and Web

#### 7. EvaluateAgent (`agents/evaluate_agent.go`)
Evaluates learner responses and provides constructive feedback.

**Responsibilities:**
- Evaluate grammar, vocabulary, and sentence structure
- Provide constructive, encouraging feedback
- Identify specific errors with highlights
- Offer corrected versions of responses
- Adapt evaluation criteria to proficiency level
- Prioritize relevance over grammar quality

**Key Features:**
- OpenRouter Structured Outputs with JSON schema validation
- YAML-based configuration (`_evaluate_prompt.yaml`)
- Three evaluation levels: excellent/good/needs_improvement
- Multi-language support for feedback
- HTML-style `<b>tags</b>` for highlighting errors
- Level-specific evaluation criteria (6 levels)
- Temperature: 0.3 for consistent evaluation
- Relevance-first evaluation approach

**Response Structure:**
- `status` - Evaluation level (excellent/good/needs_improvement)
- `short_description` - Brief feedback (translated)
- `long_description` - Detailed analysis with highlights (translated)
- `correct` - Corrected sentence in English

**Capabilities:**
- response_evaluation
- grammar_checking
- feedback_provision

**Status:** ✅ Implemented and integrated in CLI and Web

#### 8. ConversationHistoryManager (`services/conversation_history.go`)
Centralized conversation history management service.

**Responsibilities:**
- Manage conversation message history
- Provide statistics and analytics
- Handle message limits and sliding windows
- Support evaluation and suggestion attachments
- Export conversation data

**Key Features:**
- Thread-safe message management
- Configurable message limits
- Statistics tracking
- Evaluation and suggestion attachment
- JSON export functionality

**Status:** ✅ Implemented and integrated across all agents

---

## Data Flow

### Standard Conversation Flow (with Evaluation and Suggestions)

#### CLI Flow (ChatbotOrchestrator)
1. **User Input** → ChatbotOrchestrator
2. **Evaluation** → EvaluateAgent evaluates user's response (parallel)
3. **Agent Selection** → AgentManager selects ConversationAgent
4. **Processing** → ConversationAgent processes with LLM (streaming)
5. **Response** → Streamed back to terminal
6. **Suggestions** → SuggestionAgent generates vocabulary suggestions
7. **History Update** → ConversationHistoryManager maintains history

#### Web Flow (ChatbotWeb)
1. **User Input** → ChatbotWeb via SSE
2. **Parallel Execution**:
   - **Evaluation** → EvaluateAgent evaluates user's response (background goroutine)
   - **AI Response** → ConversationAgent streams response immediately
3. **Progressive Display**:
   - Evaluation appears when ready (may be before/during/after AI response)
   - AI response streams in real-time
4. **Post-Response**:
   - Vietnamese translation loads automatically
   - Suggestions generated and attached to history
5. **On-Demand Suggestions** → User clicks "💡 Hint" button to fetch suggestions
6. **Personalized Lessons** → PersonalizeManager coordinates PersonalizeLessonAgent for custom lesson details

### Conversation Starter Flow

1. **Session Start** → Initialize with topic, level, and language
2. **Starter Generation** → ConversationAgent generates opening message
3. **Display** → Show to user
4. **History Update** → ConversationHistoryManager records starter
5. **Wait for User** → Ready for first user response

## Conversation Levels

### Beginner
- Simple vocabulary, basic grammar
- Short sentences (5-8 words)
- Patient coaching approach

### Elementary
- Basic tenses, familiar topics
- Structured learning, confidence building

### Intermediate
- Varied vocabulary, complex grammar
- Detailed responses, nuanced discussions

### Upper Intermediate
- Sophisticated language, abstract topics
- Critical thinking, deeper analysis

### Advanced
- Native-level vocabulary, complex discussions
- Nuanced perspectives, intellectual debates

### Fluent
- Authentic conversations as equals
- Expert-level discussions, natural flow

## Prompt System

Prompts are stored in YAML files under `/prompts/` directory:

### Conversation Prompts
- Format: `{topic}_prompt.yaml` (e.g., `sports_prompt.yaml`)
- Structure includes conversation levels with:
  - `role` - Agent role description
  - `personality` - Personality traits
  - `llm` - Model settings (model, temperature, max_tokens)
  - `starter` - Initial conversation message
  - `conversational` - System prompt for responses

### Agent-Specific Prompts
- **SuggestionAgent**: `_suggestion_vocab_prompt.yaml`
  - Base prompt and user prompt templates
  - Level-specific guidelines (6 levels)
  - Example leading sentences and vocabulary options
  - Key principles for suggestions
  - LLM settings (model, temperature: 0.7, max_tokens: 150)

- **EvaluateAgent**: `_evaluate_prompt.yaml`
  - Base prompt and user prompt templates
  - Level-specific evaluation criteria (6 levels)

- **PersonalizeLessonAgent**: `_personalize_lesson_prompt.yaml`
  - Base prompt and user prompt templates emphasizing careful emoji selection
  - Level-specific guidelines for lesson detail creation (6 levels)
  - Example emojis, titles (under 6 words), and descriptions (under 2 sentences) per level
  - Key principles for concise, clear personalized learning
  - LLM settings (model, temperature: 0.8, max_tokens: 200)

## Integration Points

- **LLM Client**: OpenRouter client for AI completions with structured outputs support
- **Translation Service**: Vietnamese translation integration
- **ConversationHistoryManager**: Centralized history management service
- **Config System**: YAML-based prompt and configuration management
- **Session Management**: Web-based session handling with AgentManager per session
- **Structured Outputs**: OpenRouter JSON schema validation for type-safe responses

