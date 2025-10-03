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
data: {"content": "Hello", "done": false}
data: {"content": " world", "done": false}
data: {"done": true}
```

**Implementation Details:**
- Sets headers for SSE (text/event-stream)
- Uses flusher for real-time streaming
- Maintains conversation history
- Limits context to last 6 messages

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
List all available conversation topics.

**Response:**
```json
{
    "success": true,
    "topics": ["sports", "music", "technology"]
}
```

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

### 7. GET /api/prompts
List all available prompt files.

**Response:**
```json
{
    "success": true,
    "prompts": [
        {
            "name": "sports_prompt.yaml",
            "topic": "sports"
        }
    ]
}
```

### 8. GET /api/prompt/content
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

### 9. POST /api/prompt/save
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

### 10. POST /api/prompt/create
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

### 11. POST /api/prompt/delete
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

### 12. POST /api/translate
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
- Topic selection dropdown
- Level selection grid (6 levels)
- Prompt file management
  - List all prompts
  - Edit button (opens modal)
  - Delete button
  - Add new prompt button

#### 2. Chat Container
- Header with title and info
- Scrollable messages area
- Message types:
  - User messages (right-aligned, purple gradient)
  - Assistant messages (left-aligned, white with border)
  - Vietnamese translations (below assistant messages)
  - Typing indicator (animated dots)

#### 3. Input Area
- Auto-expanding textarea
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
- `createSession()` - Initialize conversation session
- `sendMessage()` - Send user message via SSE
- `editPrompt(topic)` - Open prompt editor
- `savePrompt()` - Save prompt changes
- `deletePrompt(topic)` - Delete prompt file
- `translateMessage(text)` - Get Vietnamese translation
- `validateYAML()` - Validate YAML syntax

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

1. **Streaming:** SSE for real-time response delivery
2. **History Limiting:** Max 20 messages in history, 6 for context
3. **Lazy Translation:** Translates only after full response
4. **Debounced YAML Validation:** 500ms delay prevents excessive checking
5. **Efficient DOM Updates:** Minimal reflows during streaming

## Integration with Other Components

### AgentManager
- Created per session
- Holds conversation state
- Routes to ConversationAgent

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

