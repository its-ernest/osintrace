# osintrace

Modular OSINT pipeline runner.

You define a pipeline. `osintrace` runs it. Modules do the work.

```bash
osintrace run track.yaml
```

---

## How it works

A pipeline is a YAML file. Each step is a module. A module receives an input block and a config block, does its work, prints whatever it wants to the terminal, and returns a result. That result can be passed as input to the next module.

```yaml
modules:
- name: contacts_graph_extract
  config:
    leak: ./leaks/contacts.csv

- name: contacts_graph_infer
  input:
    from: contacts_graph_extract
    artifact: graph
  config:
    model: ./models/model.onnx
    subject: ["+447911123456"]
    relation: ["+12025550505"]
```

The core does **not** interpret results. It does not cluster, score, or display anything. Modules own their logic, output, and inference.

---

## Install

Requires Go 1.22+.

```bash
# Reliable install
GOPROXY=direct go install github.com/its-ernest/osintrace/cmd/osintrace@latest

# Or simply
go install github.com/its-ernest/osintrace/cmd/osintrace@latest

# Specific version
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

**Official modules** are maintained in the `osintrace-modules` repository. Install by name:

```bash
osintrace install ip_locator
osintrace install social_patterns
```

**Community modules** live in their own repositories. Install by passing the full repo path:

```bash
osintrace install github.com/alice/osintrace-face-osint
osintrace install github.com/bob/osintrace-wifi-scanner
```

The installer detects official vs community automatically. You will be prompted before installing any unverified module.

```
  name        : face_osint
  version     : 0.1.0
  author      : alice
  description : Reverse face search across public platforms
  official    : false
  verified    : false

  ⚠  face_osint is unverified (community module). Install anyway? (y/n):
```

Installed modules are stored in `~/.osintrace/bin/`. The local registry is at `~/.osintrace/registry.json`.

Browse official and listed community modules:
[github.com/its-ernest/osintrace-modules](https://github.com/its-ernest/osintrace-modules)

---

## Pipeline reference

```yaml
modules:
  - name: <module_name>       # must be installed
    input:                    # optional, can reference a previous module
      from: previous_module
      artifact: output_name
    config:                   # passed to the module as-is
      key: value
      token: "${ENV_VAR}"     # environment variables are expanded automatically
```

Modules run sequentially in the order they are declared.

**Important:** The `$module_name` reference is no longer supported. Always use `from` + `artifact` if passing output from a prior module.

---

## Module development (Writing a modlue)

Module development is documented in [`MODULES.md`](./MODULES.md).
All module logic, output handling, and SDK usage instructions are explained there.

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

The core never knows what a module does. Modules never know they are being orchestrated. The only contract is stdin/stdout.

---

## Uninstall osintrace

```bash
# remove the binary
rm $(which osintrace)

# remove all installed modules + registry
rm -rf ~/.osintrace
```

---

## License

Mozilla Public License Version 2.0
