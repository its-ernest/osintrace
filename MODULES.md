
# Writing an opentrace Module

## What a module is

An opentrace module is a **standalone Go binary**.

It:

* receives structured input via **stdin**
* receives execution context via **environment variables**
* writes files to its **own directory**
* exits

A module **does not return data** to the pipeline.
It communicates **only through the filesystem**.

---

## Non-negotiable rules

* **stdout is ignored**
* **stderr is for humans**
* **files are the API**
* **exit code is truth**
* **never write outside your StepDir**

If you violate these rules, your module is broken by definition.

---

## Execution environment

Before your module starts, the core guarantees:

```text
OPENTRACE_RUN_DIR=/abs/path/.opentrace/runs/<run-id>
OPENTRACE_STEP_DIR=/abs/path/.opentrace/runs/<run-id>/<module-name>
```

Rules:

* `OPENTRACE_STEP_DIR` exists and is empty
* Your module **owns** this directory
* You may create any files or subdirectories inside it
* You must not write anywhere else

---

## Repository structure

```
opentrace-my-module/
├── main.go
├── go.mod
└── manifest.yaml
```

Repository name **must** be:

```
opentrace-<module_name>
```

Binary name **must** match `<module_name>`.

---

## SDK contract

### Types

```go
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
```

### Meaning

* `input.Input`
  A **string**, opaque to the core
  May be:

  * a literal
  * a file path
  * a URL
  * an identifier

* `input.Config`
  Arbitrary configuration from the pipeline YAML

* `ctx.StepDir`
  Your writable workspace

---

## Minimal module example

```go
package main

import (
	"fmt"
	"os"

	"github.com/its-ernest/opentrace/sdk"
)

type MyModule struct{}

func (m *MyModule) Name() string { return "my_module" }

func (m *MyModule) Run(input sdk.Input, ctx sdk.Context) error {
	fmt.Fprintln(os.Stderr, "input:", input.Input)
	fmt.Fprintln(os.Stderr, "working in:", ctx.StepDir)

	// write outputs here

	return nil
}

func main() {
	sdk.Run(&MyModule{})
}
```

This is a **valid module**.

---

## Reading input

`input.Input` is always a string.

### Literal input (first module)

Pipeline:

```yaml
- name: ip_locator
  input: "8.8.8.8"
```

Module:

```go
ip := input.Input // "8.8.8.8"
```

---

### Artifact input (downstream module)

Pipeline:

```yaml
- name: asn_lookup
  input:
    from: ip_locator
    artifact: result
```

Resolution:

```text
input.Input = /abs/path/.opentrace/runs/<id>/ip_locator/result.json
```

Your module **must treat this as opaque**.
Do not assume format unless documented by convention.

---

## Writing outputs

All outputs **must** be written inside `ctx.StepDir`.

Example:

```go
out := filepath.Join(ctx.StepDir, "graph.json")
os.WriteFile(out, data, 0644)
```

Nothing else is required for correctness.

---

## Declaring outputs (output.json)

If a downstream module needs your files, you **must** declare them.

Create:

```text
$OPENTRACE_STEP_DIR/output.json
```

Example:

```json
{
  "artifacts": {
    "graph": {
      "path": "graph.json",
      "type": "application/json"
    },
    "stats": {
      "path": "stats.csv",
      "type": "text/csv"
    }
  }
}
```

Rules:

* `path` is **relative to StepDir**
* Artifact names are **stable API**
* The core does **not** inspect file contents
* Multiple artifacts are allowed

This enables:

* fan-out
* caching
* inspection
* reproducibility

---

## Reading config

### Direct access

```go
threshold, _ := input.Config["threshold"].(float64)
enabled, _ := input.Config["enabled"].(bool)
```

### Structured config (recommended)

```go
//e.g
type config struct {
	ModelPath string `json:"model_path"`
	MaxDepth  int    `json:"max_depth"`
}

var cfg config
raw, _ := json.Marshal(input.Config)
json.Unmarshal(raw, &cfg)
```

---

## Printing & logging

* **Use stderr only**
* Print anything you want
* Progress bars, tables, logs are fine

```go
fmt.Fprintln(os.Stderr, "nodes:", len(nodes))
```

**Never** print to stdout.

Stdout is ignored and must not be used as a protocol.

---

## Exit behavior

* `return nil` → success
* `return error` → failure
* Non-zero exit code stops the pipeline

There is no partial success.

---

## Common mistakes (do not do these)

### !!! Writing outside StepDir

```go
os.WriteFile("/tmp/out.json", b, 0644) // wrong
```

### !!! Using stdout

```go
fmt.Println("done") // wrong
```

### !!! Returning data instead of writing files

Modules **do not return data**.
Files are the output.

---

## manifest.yaml

Every module **must** include `manifest.yaml`:

```yaml
name: graph_builder
version: 0.1.0
description: Builds a contact graph from phone metadata
author: your_handle
entity_types:
  - phone
```

This is metadata only.
It does not affect execution.

