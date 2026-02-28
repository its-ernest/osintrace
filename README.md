# osintrace

Modular OSINT pipeline runner.

You define a pipeline. osintrace runs it. Modules do the work.
```bash
osintrace run track.yaml
```

---

## How it works

A pipeline is a YAML file. Each step is a module. A module receives an input string and a config block, does its work, prints what it wants to the terminal, and returns a result string. That result can be passed as input to the next module.
```yaml
modules:
  - name: ip_locator
    input: "8.8.8.8"
    config:
      token: "${IPINFO_TOKEN}"

  - name: social_patterns
    input: "$ip_locator"     # receives ip_locator's output
    config:
      depth: 2
```

The core does not interpret results. It does not cluster, score, or display anything. Modules own their logic, their output, and their inference.

---

## Install

Requires Go 1.22+.
```bash
#Reliable install
GOPROXY=direct go install github.com/its-ernest/osintrace/cmd/osintrace@latest

#Or simply
go install github.com/its-ernest/osintrace/cmd/osintrace@latest

#Specific verrsion
GOPROXY=direct go install github.com/its-ernest/osintrace/cmd/osintrace@v0.1.4
```

Or build from source:
```bash
git clone https://github.com/its-ernest/osintrace
cd osintrace
go mod tidy
make build
```

**If the command is not found after install:**

Go installs binaries to `$GOPATH/bin`. Add it to your PATH if it isn't already.
```bash
# find your GOPATH
go env GOPATH

# add to ~/.bashrc or ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin

# reload
source ~/.bashrc   # or source ~/.zshrc
```

---

## Usage
```bash
# run a pipeline
osintrace run track.yaml

# install an official module by name
osintrace install ip_locator

# install a community module by repo
osintrace install github.com/alice/osintrace-face-osint

# uninstall a module
osintrace uninstall ip_locator

# list installed modules
osintrace modules
```

---

## Installing modules

**Official modules** are maintained in the osintrace-modules repository.
Install them by name:
```bash
osintrace install ip_locator
osintrace install social_patterns
```

**Community modules** live in their own repositories.
Install them by passing the full repo path:
```bash
osintrace install github.com/alice/osintrace-face-osint
osintrace install github.com/bob/osintrace-wifi-scanner
```

The installer detects which is which automatically.
You will be prompted before installing any unverified module.
```
  name        : face_osint
  version     : 0.1.0
  author      : alice
  description : Reverse face search across public platforms
  official    : false
  verified    : false

  ⚠  face_osint is unverified (community module). Install anyway? (y/n):
```

Installed modules are stored in `~/.osintrace/bin/`.
The local registry is at `~/.osintrace/registry.json`.

Browse official and listed community modules:
[github.com/its-ernest/osintrace-modules](https://github.com/its-ernest/osintrace-modules)

---

## Pipeline reference
```yaml
modules:
  - name: <module_name>       # must be installed
    input: "<string>"         # literal string, or $module_name to reference a prior output
    config:                   # passed to the module as-is
      key: value
      token: "${ENV_VAR}"     # env vars are expanded automatically
```

**Input referencing**

If `input` starts with `$`, it is treated as a reference to a previous module's output.
The reference must match a module name that has already run in the same pipeline.
```yaml
modules:
  - name: ip_locator
    input: "1.2.3.4"

  - name: geo_cluster
    input: "$ip_locator"      # receives whatever ip_locator returned
```

Modules run sequentially in declaration order.

---

## Writing a module

Every module is a standalone Go binary that imports the osintrace SDK,
implements one interface, and calls `sdk.Run()` in `main()`.
```go
package main

import "github.com/its-ernest/osintrace/sdk"

type MyModule struct{}

func (m *MyModule) Name() string { return "my_module" }

func (m *MyModule) Run(input sdk.Input) (sdk.Output, error) {
    // input.Input  — the string passed from the pipeline or prior module
    // input.Config — the config map from the pipeline YAML

    // print whatever you want to the terminal here

    return sdk.Output{Result: "value passed to next module"}, nil
}

func main() { sdk.Run(&MyModule{}) }
```

The SDK handles all stdin/stdout plumbing between the core and your module.
You never touch it directly.

**SDK types**
```go
type Input struct {
    Input  string         // the input string
    Config map[string]any // config block from the pipeline YAML
}

type Output struct {
    Result string // stored by core, passed as input to next module if referenced
}
```

**Your module repository structure**
```
osintrace-your-module/
├── main.go
├── go.mod
└── manifest.yaml
```
```yaml
# manifest.yaml
name: your_module
version: 0.1.0
description: What this module does
author: your_handle
official: false
verified: false
entity_types: [ip]   # ip | email | username | domain | phone | text | url
```

**Publishing**

Anyone can install your module directly from your repo without any approval:
```bash
osintrace install github.com/you/osintrace-your-module

```

To list it in the official registry so it is discoverable by name,
open a PR to [osintrace-modules](https://github.com/its-ernest/osintrace-modules)
adding one entry to `modules/registry.json`:
```json
"your_module": {
    "repo": "github.com/you/osintrace-your-module",
    "version": "0.1.0",
    "author": "you",
    "description": "What your module does",
    "official": false,
    "verified": false
}
```

That is the entire PR. One JSON block. Nobody else's code is touched.
Once listed, users can install by name:
```bash
osintrace install your_module
```

---

## Architecture
```
osintrace (core)
├── reads pipeline YAML
├── resolves module binaries from ~/.osintrace/bin/
├── runs each module as a subprocess
│   ├── sends Input as JSON over stdin
│   └── reads Output as JSON from stdout
└── passes output.Result to the next module if referenced

osintrace-modules (separate repo)
├── modules/registry.json        ← index of all official + listed community modules
└── modules/<name>/<version>/    ← official modules maintained here
    ├── main.go
    ├── go.mod
    └── manifest.yaml

community modules
└── each lives in its own repo    ← installed directly by repo path
    └── imports github.com/its-ernest/osintrace/sdk
```

The core never knows what a module does.
Modules never know they are being orchestrated.
The only contract between them is stdin and stdout.

---

## Uninstall osintrace
```bash
# remove the binary
rm $(which osintrace)

# remove all installed modules + registry
rm -rf ~/.osintrace
```

## License

Mozilla Public License Version 2.0