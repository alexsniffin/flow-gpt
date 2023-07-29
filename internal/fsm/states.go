package fsm

type Action struct {
	Type            string                 `json:"type"`
	Thought         string                 `json:"thought"`
	Output          string                 `json:"output"`
	ProblemAnalysis string                 `json:"problemAnalysis"`
	Resources       map[string]interface{} `json:"resources"`
}

type ThoughtDecider struct {
	Thought      string
	JudgeMessage string
}

type Next struct{}

type Complete struct{}

type Init struct{}

type JudgeThought struct {
	Message string
}

type JudgeAction struct {
	Problem  string
	Message  string
	AuditLog string
}
