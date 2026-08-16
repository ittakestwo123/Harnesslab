package sandbox

import (
	"context"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// maxToolOutput caps how much command output is returned to the model so a
// runaway command cannot flood the context.
const maxToolOutput = 4000

type execRequest struct {
	Command string `json:"command"`
}

type execResponse struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	Status   string `json:"status"`
}

// NewExecTool builds an exec_command tool whose commands route through the
// given sandbox. It is used in place of the host-exec toolset whenever a
// non-none sandbox is configured.
func NewExecTool(sb Sandbox, baseDir string) tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, req execRequest) (execResponse, error) {
			if strings.TrimSpace(req.Command) == "" {
				return execResponse{ExitCode: -1, Status: "error", Output: "empty command"}, nil
			}
			res := sb.Run(ctx, Command{Dir: baseDir, Command: req.Command})
			rsp := execResponse{
				ExitCode: res.ExitCode,
				Output:   clipTool(res.Output),
				Status:   "exited",
			}
			if res.Err != nil {
				rsp.Status = "error"
				if res.Output == "" {
					rsp.Output = clipTool(res.Err.Error())
				}
			}
			return rsp, nil
		},
		function.WithName("exec_command"),
		function.WithDescription(
			"Execute a shell command in the sandboxed workspace. Use this for "+
				"general local shell work such as builds, tests, and file "+
				"inspection. Commands run inside the workspace directory.",
		),
	)
}

func clipTool(s string) string {
	if len(s) <= maxToolOutput {
		return s
	}
	return s[:maxToolOutput] + "\n...[output truncated]"
}
