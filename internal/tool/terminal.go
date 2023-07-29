package tool

import (
	"context"
	"fmt"
	"reflect"

	"flow-gpt/internal/integration"
	"github.com/hupe1980/golc/schema"
)

var _ schema.Tool = (*Terminal)(nil)

type Terminal struct {
	bash *integration.BashProcess
}

func NewTerminal(bash *integration.BashProcess) *Terminal {
	return &Terminal{
		bash: bash,
	}
}

func (t *Terminal) Name() string {
	return "Terminal"
}

func (t *Terminal) Description() string {
	return `Agent will run a bash command in a headless terminal.`
}

func (t *Terminal) ArgsType() reflect.Type {
	return reflect.TypeOf("") // string
}

func (t *Terminal) Run(ctx context.Context, input any) (string, error) {
	cmd := input.(string)
	output, err := t.bash.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to run the following command=[%s]: %w", cmd, err)
	}

	return fmt.Sprintf("Successfully ran the following command=[%s] with output=[%s]", cmd, output), nil
}

func (t *Terminal) Verbose() bool {
	return false
}

func (t *Terminal) Callbacks() []schema.Callback {
	return nil
}
