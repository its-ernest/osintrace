package sdk

import (
	"encoding/json"
	"fmt"
	"os"
)

type Input struct {
	Input  string         `json:"input"`
	Config map[string]any `json:"config"`
}

type Context struct {
	RunDir  string
	StepDir string
}

type Module interface {
	Name() string
	Run(input Input, ctx Context) error
}

func Run(m Module) {
	var in Input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fatal(m, "bad input", err)
	}

	ctx := Context{
		RunDir:  os.Getenv("OPENTRACE_RUN_DIR"),
		StepDir: os.Getenv("OPENTRACE_STEP_DIR"),
	}

	if ctx.RunDir == "" || ctx.StepDir == "" {
		fatal(m, "missing runtime context", nil)
	}

	if err := os.MkdirAll(ctx.StepDir, 0o755); err != nil {
		fatal(m, "cannot create step dir", err)
	}

	if err := m.Run(in, ctx); err != nil {
		fatal(m, "module error", err)
	}
}

func fatal(m Module, msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] %s: %v\n", m.Name(), msg, err)
	} else {
		fmt.Fprintf(os.Stderr, "[%s] %s\n", m.Name(), msg)
	}
	os.Exit(1)
}