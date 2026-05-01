package adapters

type InvokeRequest struct {
	RunID        string
	NodeID       string
	Messages     []Message
	Options      InvokeOptions
	TraceContext *TraceContext
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InvokeOptions struct {
	Stream    bool
	TimeoutMs int
}

type TraceContext struct {
	ParentEventID string
}

type InvokeResult struct {
	Status   string        `json:"status"`
	Messages []Message     `json:"messages,omitempty"`
	Usage    *UsageInfo    `json:"usage,omitempty"`
	Error    *AdapterError `json:"error,omitempty"`
	RawRef   string        `json:"raw_ref,omitempty"`
}

type UsageInfo struct {
	InputTokens  int      `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	CostUSD      *float64 `json:"cost_usd,omitempty"`
}

type AdapterError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"

	ErrorAuthFailed          = "auth_failed"
	ErrorEndpointUnavailable = "endpoint_unavailable"
	ErrorTimeout             = "timeout"
	ErrorInvalidResponse     = "invalid_response"
)
