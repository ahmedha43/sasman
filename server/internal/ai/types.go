package ai

type ChatMessage struct {
	Role             string                   `json:"role"`
	Content          string                   `json:"content"`
	Name             string                   `json:"name,omitempty"`
	ToolCalls        []map[string]interface{} `json:"tool_calls,omitempty"`
	ToolCallID       string                   `json:"tool_call_id,omitempty"`
	ThoughtSignature string                   `json:"thought_signature,omitempty"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
}

type ToolCall struct {
	ID               string       `json:"id"`
	Type             string       `json:"type"`
	Function         FunctionCall `json:"function"`
	ThoughtSignature string       `json:"thought_signature,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ChatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type SecurityFinding struct {
	Category    string `json:"category"` // firewall, service, auth, interface, system
	Severity    string `json:"severity"` // critical, warning, info
	Title       string `json:"title"`
	Description string `json:"description"`
	FixPlan     string `json:"fix_plan,omitempty"`
	FixCommand  string `json:"fix_command,omitempty"`
}

type DiagnosticReport struct {
	Subdomain       string            `json:"subdomain"`
	Score           int               `json:"score"`
	OverallStatus   string            `json:"overall_status"` // excellent, good, warning, critical
	ResourceSummary map[string]string `json:"resource_summary"`
	Findings        []SecurityFinding `json:"findings"`
	MermaidTopology string            `json:"mermaid_topology,omitempty"`
	Narrative       string            `json:"narrative"`
	CreatedAt       string            `json:"created_at"`
}

type ChangePlan struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	TargetRouter string   `json:"target_router"`
	Commands     []string `json:"commands"`
	Rollback     []string `json:"rollback"`
	DiffPreview  string   `json:"diff_preview"`
	RiskLevel    string   `json:"risk_level"` // low, medium, high
}

type StreamEvent struct {
	Type      string       `json:"type"` // "thought", "tool_start", "tunnel_exec", "tool_result", "plan", "content", "error", "done"
	Title     string       `json:"title,omitempty"`
	Text      string       `json:"text,omitempty"`
	Tool      string       `json:"tool,omitempty"`
	ToolID    string       `json:"tool_id,omitempty"`
	Args      interface{}  `json:"args,omitempty"`
	Status    string       `json:"status,omitempty"` // "running", "success", "error"
	Summary   string       `json:"summary,omitempty"`
	Duration  string       `json:"duration,omitempty"`
	Plan      *ChangePlan  `json:"plan,omitempty"`
	Message   *ChatMessage `json:"message,omitempty"`
	Report    *DiagnosticReport `json:"report,omitempty"`
	Timestamp string       `json:"timestamp"`
}
