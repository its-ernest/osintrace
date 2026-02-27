package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	modulesRepo   = "https://github.com/its-ernest/opentrace-modules"
	modulesPrefix = "modules"
)

type Manifest struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Official    bool     `yaml:"official"`
	Verified    bool     `yaml:"verified"`
	EntityTypes []string `yaml:"entity_types"`
	Repo        string   `yaml:"repo"`
}

type RegistryEntry struct {
	BinPath  string `json:"bin_path"`
	Version  string `json:"version"`
	Author   string `json:"author"`
	Official bool   `json:"official"`
	Verified bool   `json:"verified"`
}

type Registry map[string]RegistryEntry

func home() string         { h, _ := os.UserHomeDir(); return h }
func BinDir() string       { return filepath.Join(home(), ".opentrace", "bin") }
func registryPath() string { return filepath.Join(home(), ".opentrace", "registry.json") }

func LoadRegistry() Registry {
	r := Registry{}
	data, err := os.ReadFile(registryPath())
	if err != nil {
		return r
	}
	_ = json.Unmarshal(data, &r)
	return r
}

func saveRegistry(r Registry) error {
	_ = os.MkdirAll(filepath.Dir(registryPath()), 0o755)
	data, _ := json.MarshalIndent(r, "", "  ")
	return os.WriteFile(registryPath(), data, 0o644)
}

// Install is the single entry point.
// Detects whether the argument is a name (official) or a repo path (external).
func Install(arg string) error {
	if err := os.MkdirAll(BinDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if isExternalRepo(arg) {
		return installExternal(arg)
	}
	return installOfficial(arg)
}

// isExternalRepo returns true if the argument looks like a repo path.
// e.g. github.com/alice/opentrace-face-osint
func isExternalRepo(arg string) bool {
	return strings.Contains(arg, "/")
}

// installOfficial fetches from opentrace-modules using sparse checkout.
func installOfficial(name string) error {
	tmp, err := os.MkdirTemp("", "opentrace-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fmt.Printf("  fetching %s from opentrace-modules...\n", name)

	sparseDir := filepath.Join(modulesPrefix, name)

	if out, err := exec.Command("git", "clone",
		"--depth=1", "--filter=blob:none", "--sparse",
		modulesRepo, tmp,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s: %w", string(out), err)
	}

	if out, err := exec.Command("git", "-C", tmp,
		"sparse-checkout", "set", sparseDir,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("sparse-checkout: %s: %w", string(out), err)
	}

	moduleDir := filepath.Join(tmp, modulesPrefix, name)
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		return fmt.Errorf("module %q not found in opentrace-modules", name)
	}

	version, err := latestVersion(moduleDir)
	if err != nil {
		return fmt.Errorf("no versions found for %q: %w", name, err)
	}

	srcDir := filepath.Join(moduleDir, version)

	manifest, err := readManifest(filepath.Join(srcDir, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	printManifest(manifest)

	// official modules are always verified — no prompt needed
	return build(name, version, srcDir, manifest, true)
}

// installExternal clones a community repo, reads its manifest, prompts if unverified.
// arg is the full repo path e.g. github.com/alice/opentrace-face-osint
func installExternal(arg string) error {
	// derive module name from last path segment
	// github.com/alice/opentrace-face-osint → opentrace-face-osint
	// then strip opentrace- prefix if present for the bin name
	repoName := arg[strings.LastIndex(arg, "/")+1:]
	name := strings.TrimPrefix(repoName, "opentrace-")

	repoURL := "https://" + arg
	// handle if they already passed https://
	if strings.HasPrefix(arg, "https://") {
		repoURL = arg
	}

	tmp, err := os.MkdirTemp("", "opentrace-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fmt.Printf("  fetching %s...\n", arg)

	if out, err := exec.Command("git", "clone",
		"--depth=1", repoURL, tmp,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s: %w", string(out), err)
	}

	// manifest must be at root of the repo
	manifest, err := readManifest(filepath.Join(tmp, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	// use manifest name as the canonical module name if available
	if manifest.Name != "" {
		name = manifest.Name
	}

	printManifest(manifest)

	// external repos are always unverified unless explicitly marked
	if !manifest.Verified {
		fmt.Printf("  ⚠  %s is unverified (community module). Install anyway? (y/n): ", name)
		var confirm string
		fmt.Scan(&confirm)
		if confirm != "y" {
			fmt.Println("  aborted.")
			return nil
		}
	}

	return build(name, manifest.Version, tmp, manifest, false)
}

// build compiles the module source and registers it.
func build(name, version, srcDir string, manifest *Manifest, official bool) error {
	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(BinDir(), binName)

	fmt.Printf("  building %s@%s...\n", name, version)
	if out, err := exec.Command("go", "build", "-trimpath", "-o", binPath, srcDir).CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %s: %w", string(out), err)
	}

	reg := LoadRegistry()
	reg[name] = RegistryEntry{
		BinPath:  binPath,
		Version:  manifest.Version,
		Author:   manifest.Author,
		Official: official,
		Verified: manifest.Verified,
	}
	if err := saveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("  ✓ %s@%s installed → %s\n", name, version, binPath)
	return nil
}

func Uninstall(name string) error {
	reg := LoadRegistry()
	entry, ok := reg[name]
	if !ok {
		return fmt.Errorf("module %q is not installed", name)
	}
	_ = os.Remove(entry.BinPath)
	delete(reg, name)
	return saveRegistry(reg)
}

func List() {
	reg := LoadRegistry()
	if len(reg) == 0 {
		fmt.Println("  no modules installed — run: opentrace install <name>")
		return
	}
	fmt.Println()
	fmt.Printf("  %-22s  %-10s  %-16s  %s\n", "MODULE", "VERSION", "AUTHOR", "STATUS")
	fmt.Printf("  %-22s  %-10s  %-16s  %s\n",
		"──────────────────────", "─────────", "───────────────", "──────────")
	for name, entry := range reg {
		status := "unverified"
		if entry.Official {
			status = "official"
		} else if entry.Verified {
			status = "verified"
		}
		fmt.Printf("  %-22s  %-10s  %-16s  %s\n",
			name, entry.Version, entry.Author, status)
	}
	fmt.Println()
}

func BinPath(name string) (string, error) {
	reg := LoadRegistry()
	entry, ok := reg[name]
	if !ok {
		return "", fmt.Errorf("module %q is not installed", name)
	}
	return entry.BinPath, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printManifest(m *Manifest) {
	fmt.Println()
	fmt.Printf("  name        : %s\n", m.Name)
	fmt.Printf("  version     : %s\n", m.Version)
	fmt.Printf("  author      : %s\n", m.Author)
	fmt.Printf("  description : %s\n", m.Description)
	fmt.Printf("  official    : %v\n", m.Official)
	fmt.Printf("  verified    : %v\n", m.Verified)
	fmt.Println()
}

func latestVersion(moduleDir string) (string, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return "", err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no version directories found")
	}
	return versions[len(versions)-1], nil
}

func readManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read manifest at %q: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest YAML: %w", err)
	}
	if m.Name == "" || m.Version == "" {
		return nil, fmt.Errorf("manifest missing required fields: name and version")
	}
	return &m, nil
}