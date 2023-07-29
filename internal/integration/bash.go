package integration

import (
	"context"
	"fmt"
	"os/exec"
)

type BashProcessError struct {
	Output       string
	ProcessState string
	Err          error
}

func (bpe BashProcessError) Error() string {
	return fmt.Sprintf("output=[%s], process state=[%s], error=[%s]", bpe.Output, bpe.ProcessState, bpe.Err)
}

type BashProcess struct{}

func NewBashProcess() *BashProcess {
	return &BashProcess{}
}

func (bp *BashProcess) Run(ctx context.Context, command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)

	output, err := cmd.CombinedOutput()
	if err != nil || cmd.ProcessState.ExitCode() != 0 {
		return "", BashProcessError{
			Output:       string(output),
			ProcessState: cmd.ProcessState.String(),
			Err:          err,
		}
	}

	return string(output), nil
}
