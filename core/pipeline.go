package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

/*
PIPELINE MODEL

- The core orchestrates execution only
- Modules communicate exclusively via the filesystem
- Stdout is ignored
- Stderr is operator-facing
- Exit code is truth
*/

type Pipeline struct {
	Modules []Step `yaml:"modules"`
}

type Step struct {
	Name   string         `yaml:"name"`
	Input  any            `yaml:"input"`  // string OR map (artifact reference)
	Config map[string]any `yaml:"config"`
}

/*
INPUT FORMS

1. Literal:
   input: "contacts.csv"

2. Artifact reference:
   input:
     from: contacts_graph
     artifact: graph
*/

type artifactRef struct {
	From     string `yaml:"from"`
	Artifact string `yaml:"artifact"`
}

/*
OUTPUT INDEX (written by modules optionally)

$STEP_DIR/output.json
{
  "artifacts": {
    "graph": {
      "path": "graph.json",
      "type": "application/json"
    }
  }
}
*/

type outputIndex struct {
	Artifacts map[string]struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"artifacts"`
}

// Load parses a pipeline YAML file
func Load(path string) (*Pipeline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Pipeline
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, err
	}

	if len(p.Modules) == 0 {
		return nil, fmt.Errorf("pipeline has no modules")
	}

	return &p, nil
}

// Run executes the pipeline
func Run(ctx context.Context, p *Pipeline, binDir string) error {
	runDir, err := os.MkdirTemp("", "opentrace-run-*")
	if err != nil {
		return err
	}

	for _, step := range p.Modules {
		stepDir := filepath.Join(runDir, step.Name)

		if err := os.MkdirAll(stepDir, 0o755); err != nil {
			return err
		}

		input, err := resolveInput(runDir, step.Input)
		if err != nil {
			return fmt.Errorf("[%s] input resolution failed: %w", step.Name, err)
		}

		if err := runModule(
			ctx,
			filepath.Join(binDir, step.Name),
			input,
			step.Config,
			runDir,
			stepDir,
		); err != nil {
			return fmt.Errorf("[%s] %w", step.Name, err)
		}
	}

	return nil
}

// resolveInput converts pipeline input into a literal or absolute artifact path
func resolveInput(runDir string, raw any) (string, error) {
	if raw == nil {
		return "", nil
	}

	// Literal string
	if v, ok := raw.(string); ok {
		return v, nil
	}

	// Artifact reference
	var ref artifactRef
	b, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	if err := yaml.Unmarshal(b, &ref); err != nil {
		return "", err
	}

	if ref.From == "" || ref.Artifact == "" {
		return "", fmt.Errorf("invalid artifact reference")
	}

	indexPath := filepath.Join(runDir, ref.From, "output.json")
	rawIndex, err := os.ReadFile(indexPath)
	if err != nil {
		return "", fmt.Errorf("missing output.json for step %s", ref.From)
	}

	var idx outputIndex
	if err := json.Unmarshal(rawIndex, &idx); err != nil {
		return "", fmt.Errorf("invalid output.json for step %s", ref.From)
	}

	art, ok := idx.Artifacts[ref.Artifact]
	if !ok {
		return "", fmt.Errorf(
			"artifact %q not found in %s/output.json",
			ref.Artifact,
			ref.From,
		)
	}

	return filepath.Join(runDir, ref.From, art.Path), nil
}

// runModule executes a module binary
func runModule(
	ctx context.Context,
	binPath string,
	input string,
	config map[string]any,
	runDir string,
	stepDir string,
) error {

	payload := map[string]any{
		"input":  input,
		"config": config,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stdout = nil           // stdout is ignored by design
	cmd.Stderr = os.Stderr    // operator UX
	cmd.Env = append(os.Environ(),
		"OPENTRACE_RUN_DIR="+runDir,
		"OPENTRACE_STEP_DIR="+stepDir,
	)

	return cmd.Run()
}