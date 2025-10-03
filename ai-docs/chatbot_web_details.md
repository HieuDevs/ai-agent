# ChatbotWeb Detailed Documentation

## Overview
ChatbotWeb provides a complete web-based interface for the English conversation chatbot. It includes both backend API endpoints and a fully embedded frontend HTML/CSS/JavaScript application.

## Architecture

### Main Structure
```go
type ChatbotWeb struct {
    manager *AgentManager
    mu      sync.Mutex
    apiKey  string
}
```

**Fields:**
- `manager` - Reference to AgentManager for conversation handling
- `mu` - Mutex for thread-safe session management
- `apiKey` - OpenRouter API key

### Request/Response Types

#### ChatRequest
```go
type ChatRequest struct {
    Message string `json:"message"`
    Action  string `json:"action"`
    Topic   string `json:"topic,omitempty"`
    Level   string `json:"level,omitempty"`
}
```

#### ChatResponse
```go
type ChatResponse struct {
    Success bool          `json:"success"`
    Message string        `json:"message"`
    Stats   interface{}   `json:"stats,omitempty"`
    Level   string        `json:"level,omitempty"`
    Topic   string        `json:"topic,omitempty"`
    Topics  []string      `json:"topics,omitempty"`
    History []ChatMessage `json:"history,omitempty"`
    Prompts []PromptInfo  `json:"prompts,omitempty"`
    Content string        `json:"content,omitempty"`
}
```

## API Endpoints

### 1. GET /
Serves the embedded HTML chat interface.
- **Response:** Full HTML page with inline CSS and JavaScript
- **Features:** Complete chat UI with all client-side functionality

### 2. POST /api/chat
Handles various chat actions.

**Actions:**
- `init` - Initialize conversation with starter message
- `stats` - Get conversation statistics
- `reset` - Reset conversation history and restart
- `set_level` - Change conversation level
- `history` - Retrieve full conversation history

**Example Request:**
```json
{
    "action": "reset"
}
```

**Example Response:**
```json
{
    "success": true,
    "message": "Hello! Let's talk!",
    "stats": {
        "total_messages": 2,
        "user_messages": 1,
        "bot_messages": 1
    }
}
```

### 3. GET /api/stream
Server-sent events endpoint for streaming AI responses.

**Query Parameters:**
- `message` - User's message (URL encoded)

**Response Format:**
```javascript
data: {"type": "evaluation", "data": {...}, "done": false}
data: {"type": "message", "content": "Hello", "done": false}
data: {"type": "message", "content": " world", "done": false}
data: {"type": "done", "done": true}
```

**Event Types:**
1. **evaluation** - User message evaluation (appears in parallel with message streaming)
   - Contains: `status`, `short_description`, `long_description`, `correct`
   - Evaluates the user's message immediately after submission
2. **message** - Streaming AI response chunks
   - Contains: `content` field with partial text
3. **done** - Final event marking completion

**Note:** Suggestions are no longer sent via SSE. They are now fetched on-demand via `/api/suggestions` endpoint.

**Implementation Details:**
- Sets headers for SSE (text/event-stream)
- Uses flusher for real-time streaming
- Evaluates user message in parallel goroutine (non-blocking)
- Evaluation can arrive at any time during streaming via select statement
- Maintains conversation history
- Limits context to last 6 messages

**Performance Features:**
- **Parallel execution**: Evaluation runs in background goroutine while AI streams
- **Non-blocking**: AI response starts immediately without waiting
- **Multi-channel select**: Handles evaluation, streaming, and completion events simultaneously
- **Progressive rendering**: Evaluation appears as soon as ready, may be before/during/after AI response

### 4. POST /api/init
Initialize session state.

**Response:**
```json
{
    "success": true,
    "level": "intermediate",
    "topic": "Sports",
    "stats": {
        "total_messages": 0,
        "user_messages": 0,
        "bot_messages": 0
    }
}
```

### 5. GET /api/topics
List all available conversation topics (excludes system prompts starting with `_`).

**Response:**
```json
{
    "success": true,
    "topics": ["sports", "music", "technology"]
}
```

**Note:** This endpoint only returns user-facing topics. System prompts (starting with `_`) like `_evaluate_prompt.yaml` and `_suggestion_vocab_prompt.yaml` are excluded from the topic dropdown.

**Implementation:**
- Scans `/prompts/` directory
- Looks for files matching `*_prompt.yaml`
- Extracts topic names

### 6. POST /api/create-session
Create a new conversation session with specific topic and level.

**Request:**
```json
{
    "topic": "sports",
    "level": "intermediate"
}
```

**Response:**
```json
{
    "success": true,
    "message": "Let's talk about sports!",
    "stats": {...},
    "level": "intermediate",
    "topic": "Sports"
}
```

**Features:**
- Includes starter message from conversation agent
- User can click "💡 Hint" button (in input area next to Send button) to get suggestions on-demand

### 7. POST /api/suggestions
Get vocabulary suggestions on-demand for a specific AI message.

**Request:**
```json
{
    "message": "Hello! What's your favorite sport?"
}
```

**Response:**
```json
{
    "success": true,
    "suggestions": {
        "leading_sentence": "You can respond with...",
        "vocab_options": [
            {"text": "I love playing basketball", "emoji": "🏀"},
            {"text": "I enjoy watching soccer", "emoji": "⚽"}
        ]
    }
}
```

**Features:**
- Only called when user clicks "💡 Hint" button (in input area)
- Button is located next to the Send button for easy access
- Fetches suggestions based on the LAST bot message content
- Displays clickable vocabulary options below the last bot message
- Removes previous suggestions if hint button clicked again

### 8. GET /api/prompts
List all available prompt files (including system prompts starting with `_`).

**Response:**
```json
{
    "success": true,
    "prompts": [
        {
            "name": "sports_prompt.yaml",
            "topic": "sports"
        },
        {
            "name": "_evaluate_prompt.yaml",
            "topic": "_evaluate"
        },
        {
            "name": "_suggestion_vocab_prompt.yaml",
            "topic": "_suggestion_vocab"
        }
    ]
}
```

**Note:** This endpoint returns ALL prompt files, including system prompts starting with `_`. These are shown in the Prompt Files section for editing.

### 9. GET /api/prompt/content
Get content of a specific prompt file.

**Query Parameters:**
- `topic` - Topic name

**Response:**
```json
{
    "success": true,
    "content": "conversation_levels:\n  beginner:..."
}
```

### 10. POST /api/prompt/save
Save edited prompt file.

**Request:**
```json
{
    "topic": "sports",
    "content": "conversation_levels:..."
}
```

**Response:**
```json
{
    "success": true,
    "message": "Prompt saved and conversation reset"
}
```

**Behavior:**
- Saves prompt to file
- If current session uses edited topic, resets conversation

### 11. POST /api/prompt/create
Create new prompt file.

**Request:**
```json
{
    "topic": "cooking",
    "content": ""
}
```

**Response:**
```json
{
    "success": true,
    "message": "Prompt file created successfully",
    "topic": "cooking"
}
```

**Features:**
- If content is empty, generates default template
- Validates topic doesn't already exist

### 12. POST /api/prompt/delete
Delete a prompt file.

**Request:**
```json
{
    "topic": "cooking"
}
```

**Response:**
```json
{
    "success": true,
    "message": "Prompt file deleted successfully"
}
```

### 13. POST /api/translate
Translate text to Vietnamese.

**Request:**
```json
{
    "text": "How are you doing today?"
}
```

**Response:**
```json
{
    "success": true,
    "content": "Bạn hôm nay thế nào?"
}
```

## Frontend Features

### UI Components

#### 1. Sidebar
- Topic selection dropdown (excludes system prompts starting with `_`)
- Level selection grid (6 levels)
- Prompt file management (shows ALL files including system prompts)
  - List all prompts (including `_evaluate_prompt.yaml`, `_suggestion_vocab_prompt.yaml`)
  - Edit button (opens modal)
  - Delete button
  - Add new prompt button

#### 2. Chat Container
- Header with title and info
- Scrollable messages area
- Message types:
  - User messages (right-aligned, purple gradient)
    - Evaluation box appears below user message (blue theme)
    - Shows status emoji (✨ excellent, 👍 good, 📚 needs improvement)
    - Displays short feedback, detailed analysis, and corrected version
  - Assistant messages (left-aligned, white with border)
    - Vietnamese translations (below assistant messages)
    - Suggestions box appears below last bot message when hint button clicked (green theme)
    - Shows leading sentence and clickable vocabulary options
  - Typing indicator (animated dots)

#### 3. Input Area
- Auto-expanding textarea
- "💡 Hint" button (green, next to Send button)
- Send button (disabled when no session or sending)
- Enter to send (Shift+Enter for new line)

#### 4. Prompt Editor Modal
- YAML editor with syntax highlighting
- Real-time YAML validation
- Error display
- Save/Cancel buttons
- Topic name input (for new prompts)

### JavaScript Features

**State Management:**
- `currentTopic` - Selected topic
- `currentLevel` - Selected level
- `sessionActive` - Session status
- `isSending` - Prevent double sends

**Key Functions:**
- `init()` - Load topics and prompts on page load
- `loadTopics()` - Fetch available topics
- `loadPrompts()` - Fetch prompt files
- `createSession()` - Initialize conversation session, display starter message
- `sendMessage()` - Send user message via SSE
- `showHint()` - Fetch and display suggestions for the last bot message when hint button is clicked
- `addMessage(role, content, translation)` - Add message to chat, automatically adds translation for assistant messages
- `editPrompt(topic)` - Open prompt editor
- `savePrompt()` - Save prompt changes
- `deletePrompt(topic)` - Delete prompt file
- `translateMessage(text, translationDiv)` - Get Vietnamese translation and update translation div
- `validateYAML()` - Validate YAML syntax
- `useSuggestion(text)` - Auto-fill input with suggestion (strips emojis automatically)

**YAML Validation:**
- Uses js-yaml library
- Real-time validation on input
- 500ms debounce
- Visual error indication

### Styling

**Color Scheme:**
- Primary gradient: #667eea → #764ba2
- Background: #f5f5f5
- White cards with subtle borders
- Success: #4CAF50
- Error: #f44336

**Responsive Design:**
- Sidebar: 320px (280px on mobile)
- Flexible chat container
- Touch-friendly controls
- Mobile-optimized layout

## Security Considerations

1. **Thread Safety:** Mutex protects manager access during concurrent requests
2. **Input Validation:** All inputs validated before processing
3. **File Operations:** Checks for file existence before operations
4. **YAML Validation:** Client-side and server-side validation
5. **Error Handling:** Graceful error responses

## Performance Optimizations

1. **Parallel Execution:** Evaluation runs in goroutine while AI streams (no blocking)
2. **Streaming:** SSE for real-time response delivery
3. **Multi-channel Select:** Handles evaluation, streaming, completion simultaneously
4. **History Limiting:** Max 20 messages in history, 6 for context
5. **Lazy Translation:** Translates only after full response
6. **Debounced YAML Validation:** 500ms delay prevents excessive checking
7. **Efficient DOM Updates:** Minimal reflows during streaming
8. **Progressive Enhancement:** Evaluation appears when ready, doesn't block AI response

## Conversation Flow

### Initial Session Creation

When user selects topic and level:

1. **Session Initialization:**
   - Create new conversation session
   - ConversationAgent generates starter message
   - Message returned in API response

2. **UI Display:**
   - Starter message appears (white, left-aligned)
   - Vietnamese translation loads automatically
   - "💡 Hint" button in input area becomes enabled

### Subsequent User Interactions

For each user message after the starter:

1. **User Input Phase:**
   - User types message and presses Send/Enter
   - User message displayed immediately (right-aligned, purple)

2. **Parallel Execution Phase (Performance Optimized):**
   - **Evaluation** runs in parallel goroutine (non-blocking)
     - EvaluateAgent evaluates user's message in background
     - Can arrive at any time during AI streaming
   - **AI Response** starts immediately (does not wait for evaluation)
     - Typing indicator appears
     - ConversationAgent streams response in real-time
     - Text appears word-by-word

3. **Evaluation Display Phase:**
   - Evaluation box appears below user message when ready (blue theme)
   - Shows status emoji, feedback, and corrected version
   - May appear before, during, or after AI response streaming

4. **Post-Response Phase:**
   - Vietnamese translation loads automatically after AI response completes
   - User can click "💡 Hint" button (in input area) anytime:
     - Button shows "⏳ Loading..." state
     - Fetches suggestions for the LAST bot message
     - Suggestions box appears below the last bot message (green theme)
     - Shows leading sentence and clickable options
     - User can click any suggestion to auto-fill input

This flow ensures:
- **User control** - suggestions only appear when user requests them
- **Maximum performance** - evaluation and AI response run in parallel
- **No blocking** - user gets immediate AI response without waiting
- **Progressive enhancement** - evaluation appears when ready
- **Easy access** - hint button always visible in input area, works with latest bot message

## Integration with Other Components

### AgentManager
- Created per session
- Holds conversation state
- Routes to ConversationAgent, EvaluateAgent, SuggestionAgent

### EvaluateAgent
- Evaluates user messages in parallel goroutine (non-blocking)
- Runs simultaneously with AI response streaming for maximum performance
- Uses structured outputs for consistent JSON format
- Provides status, descriptions, and corrections
- Results sent via channel when ready (progressive enhancement)

### SuggestionAgent
- Generates suggestions on-demand when user clicks "💡 Hint" button (in input area)
- Always fetches suggestions for the LAST bot message (represents current conversation context)
- Based on the specific AI message content passed in request
- Uses structured outputs with emojis
- Provides clickable vocabulary options
- Removes previous suggestions if hint button clicked again
- Works independently for each message (no conversation history required)

### Translation Service
- Called for all assistant messages
- Displayed below English response
- Loading indicator during translation

### Export Utility
- Not directly integrated in web version
- Available in CLI version

### Prompt Configuration
- Dynamically loads from `/prompts/` directory
- YAML-based format
- Per-level configuration

