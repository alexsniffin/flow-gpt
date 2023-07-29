package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/hupe1980/golc/schema"
)

type CallbackAuditLog struct {
	audit []string
}

func NewCallbackAuditLog() *CallbackAuditLog {
	return &CallbackAuditLog{
		audit: []string{},
	}
}

func (mc *CallbackAuditLog) AuditLog() string {
	for i, str := range mc.audit {
		mc.audit[i] = fmt.Sprintf("[LOG-%d] %s", i, str)
	}

	return strings.Join(mc.audit, "\n")
}

func (mc *CallbackAuditLog) AlwaysVerbose() bool {
	return true
}

func (mc *CallbackAuditLog) RaiseError() bool {
	return false
}

func (mc *CallbackAuditLog) OnLLMStart(ctx context.Context, input *schema.LLMStartInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnChatModelStart(ctx context.Context, input *schema.ChatModelStartInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnModelNewToken(ctx context.Context, input *schema.ModelNewTokenInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnModelEnd(ctx context.Context, input *schema.ModelEndInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnModelError(ctx context.Context, input *schema.ModelErrorInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnChainStart(ctx context.Context, input *schema.ChainStartInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnChainEnd(ctx context.Context, input *schema.ChainEndInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnChainError(ctx context.Context, input *schema.ChainErrorInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnAgentAction(ctx context.Context, input *schema.AgentActionInput) error {
	mc.audit = append(mc.audit, fmt.Sprintf("[AGENT] log_entry=[%v]", input.Action.Log))
	return nil
}

func (mc *CallbackAuditLog) OnAgentFinish(ctx context.Context, input *schema.AgentFinishInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnToolStart(ctx context.Context, input *schema.ToolStartInput) error {
	mc.audit = append(mc.audit, fmt.Sprintf("[TOOL] name=[%s] input=[%s]", input.ToolName, input.Input))
	return nil
}

func (mc *CallbackAuditLog) OnToolEnd(ctx context.Context, input *schema.ToolEndInput) error {
	mc.audit = append(mc.audit, fmt.Sprintf("[TOOL] output=[%s]", input.Output))
	return nil
}

func (mc *CallbackAuditLog) OnToolError(ctx context.Context, input *schema.ToolErrorInput) error {
	mc.audit = append(mc.audit, fmt.Sprintf("[TOOL] error=[%s]", input.Error))
	return nil
}

func (mc *CallbackAuditLog) OnText(ctx context.Context, input *schema.TextInput) error {
	mc.audit = append(mc.audit, fmt.Sprintf("[TEXT] log_entry=[%s]", input.Text))
	return nil
}

func (mc *CallbackAuditLog) OnRetrieverStart(ctx context.Context, input *schema.RetrieverStartInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnRetrieverEnd(ctx context.Context, input *schema.RetrieverEndInput) error {
	return nil
}

func (mc *CallbackAuditLog) OnRetrieverError(ctx context.Context, input *schema.RetrieverErrorInput) error {
	return nil
}
