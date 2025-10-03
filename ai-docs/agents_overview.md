# Agents Architecture Overview

## Agent System Components

The agents package contains a modular AI agent system for an English conversation learning application. The system is designed to help users practice English at different proficiency levels.

### Core Components

#### 1. AgentManager (`manager.go`)
Central orchestrator for all agents in the system.

**Responsibilities:**
- Register and manage available agents
- Route tasks to appropriate agents
- Maintain client connections
- Process job requests

**Key Methods:**
- `NewManager(apiKey, level, topic)` - Initialize manager with conversation parameters
- `RegisterAgents(level, topic)` - Register all available agents
- `SelectAgent(task)` - Choose appropriate agent for task
- `ProcessJob(job)` - Execute job with selected agent
- `GetAgent(name)` - Retrieve specific agent by name

#### 2. ConversationAgent (`conversation_agent.go`)
Main agent handling English conversation practice.

**Responsibilities:**
- Manage conversation flow and context
- Generate level-appropriate responses
- Maintain conversation history
- Support streaming responses
- Integrate Vietnamese translations

**Key Features:**
- Support for 6 proficiency levels (beginner to fluent)
- Dynamic prompt loading based on level and topic
- Conversation history management (max 20 messages)
- Recent history windowing (last 6 messages for context)
- LLM settings per level (model, temperature, max_tokens)

**Capabilities:**
- english_conversation
- teaching_response
- conversation_starter
- contextual_responses
- level_appropriate_challenge
- Level-specific capabilities (vocabulary, grammar, discussion complexity)

**Key Methods:**
- `ProcessTask(task)` - Handle conversation task
- `generateConversationStarter()` - Create initial message
- `generateConversationalResponse()` - Generate contextual replies
- `SetLevel(level)` - Change conversation difficulty
- `ResetConversation()` - Clear history
- `GetConversationStats()` - Return message statistics

#### 3. ChatbotOrchestrator (`chatbot_orchestrator.go`)
Terminal-based interactive conversation interface.

**Responsibilities:**
- CLI-based conversation sessions
- User command processing
- Session lifecycle management
- Statistics and history display

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

#### 4. ChatbotWeb (`chatbot_web.go`)
Web-based conversation interface with full UI.

**Responsibilities:**
- HTTP server for web interface
- RESTful API endpoints
- Server-sent events for streaming
- Prompt file management
- Translation services

**API Endpoints:**
- `GET /` - Serve chat HTML interface
- `POST /api/chat` - Handle chat actions (init, stats, reset, set_level, history)
- `GET /api/stream` - Stream AI responses (SSE)
- `POST /api/init` - Initialize session state
- `GET /api/topics` - List available topics
- `POST /api/create-session` - Create new session with topic/level
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

#### 5. SuggestionAgent (`suggestion_agent.go`)
Provides vocabulary suggestions and sentence starters to help learners respond.

**Responsibilities:**
- Generate contextual vocabulary suggestions
- Provide sentence structure guidance
- Offer emoji-enhanced vocabulary options
- Adapt to learner's proficiency level

**Key Features:**
- OpenRouter Structured Outputs with JSON schema validation
- YAML-based configuration (`_suggestion_vocab_prompt.yaml`)
- Three vocabulary options per suggestion
- Multi-language support for leading sentences
- Level-adaptive prompts (6 levels)
- Emoji integration for visual enhancement

**Response Structure:**
- `leading_sentence` - Guide for response structure (translated)
- `vocab_options` - Three options with text and emoji

**Capabilities:**
- vocabulary_suggestion
- response_guidance
- sentence_completion

**Status:** ✅ Implemented and integrated in CLI

#### 6. EvaluateAgent (`evaluate_agent.go`)
Evaluates learner responses and provides constructive feedback.

**Responsibilities:**
- Evaluate grammar, vocabulary, and sentence structure
- Provide constructive, encouraging feedback
- Identify specific errors with highlights
- Offer corrected versions of responses
- Adapt evaluation criteria to proficiency level

**Key Features:**
- OpenRouter Structured Outputs with JSON schema validation
- YAML-based configuration (`_evaluate_prompt.yaml`)
- Three evaluation levels: excellent/good/needs_improvement
- Multi-language support for feedback
- HTML-style `<b>tags</b>` for highlighting errors
- Level-specific evaluation criteria (6 levels)
- Lower temperature (0.3) for consistent evaluation

**Response Structure:**
- `status` - Evaluation level (excellent/good/needs_improvement)
- `short_description` - Brief feedback (translated)
- `long_description` - Detailed analysis with highlights (translated)
- `correct` - Corrected sentence in English

**Capabilities:**
- response_evaluation
- grammar_checking
- feedback_provision

**Status:** ✅ Implemented and integrated in CLI

---

## Data Flow

### Standard Conversation Flow (with Evaluation and Suggestions)

1. **User Input** → ChatbotOrchestrator/ChatbotWeb
2. **Task Creation** → JobRequest with user message
3. **Evaluation** (optional) → EvaluateAgent evaluates user's response
   - Provides feedback on grammar, vocabulary, structure
   - Shows corrected version if needed
4. **Agent Selection** → AgentManager selects ConversationAgent
5. **Processing** → ConversationAgent processes with LLM
6. **Response** → Streamed back to user interface
7. **Suggestions** → SuggestionAgent generates vocabulary suggestions
   - Based on AI's last message
   - Provides leading sentence and vocabulary options
8. **History Update** → Conversation history maintained

### Conversation Starter Flow

1. **Session Start** → Initialize with topic and level
2. **Starter Generation** → ConversationAgent generates opening message
3. **Display** → Show to user
4. **Suggestions** → SuggestionAgent provides initial vocabulary help
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
  - Guidelines for each proficiency level
  - Key principles for constructive feedback
  - LLM settings (model, temperature: 0.3, max_tokens: 500)

## Integration Points

- **LLM Client**: OpenRouter client for AI completions
- **Translation Service**: Vietnamese translation integration
- **Export Utility**: JSON export for conversation history
- **Config System**: YAML-based prompt and configuration management

