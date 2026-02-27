package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const (
	modulesRepo    = "https://github.com/its-ernest/opentrace-modules"
	modulesPrefix  = "modules" // modules/<name>/<version>/
)

// Manifest is the parsed manifest.yaml from the module directory.
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

// RegistryEntry is what gets stored locally after install.
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

// Install fetches a module by name, reads its manifest, prompts if unverified,
// builds the binary, and registers it locally.
func Install(name string) error {
	if err := os.MkdirAll(BinDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "opentrace-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Step 1: sparse clone - pull only modules/<name>/ from the repo
	fmt.Printf("  fetching %s...\n", name)

	sparseDir := filepath.Join(modulesPrefix, name)

	if out, err := exec.Command("git", "clone",
		"--depth=1",
		"--filter=blob:none",
		"--sparse",
		modulesRepo, tmp,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s: %w", string(out), err)
	}

	if out, err := exec.Command("git", "-C", tmp,
		"sparse-checkout", "set", sparseDir,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("sparse-checkout: %s: %w", string(out), err)
	}

	// Step 2: find latest version directory
	// modules/ip_locator/ may contain multiple version dirs — pick latest
	moduleDir := filepath.Join(tmp, modulesPrefix, name)
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		return fmt.Errorf("module %q not found in opentrace-modules", name)
	}

	version, err := latestVersion(moduleDir)
	if err != nil {
		return fmt.Errorf("no versions found for %q: %w", name, err)
	}

	srcDir := filepath.Join(moduleDir, version)

	// Step 3: read and parse manifest.yaml
	manifest, err := readManifest(filepath.Join(srcDir, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	// Step 4: display module info
	fmt.Println()
	fmt.Printf("  name        : %s\n", manifest.Name)
	fmt.Printf("  version     : %s\n", manifest.Version)
	fmt.Printf("  author      : %s\n", manifest.Author)
	fmt.Printf("  description : %s\n", manifest.Description)
	fmt.Printf("  official    : %v\n", manifest.Official)
	fmt.Printf("  verified    : %v\n", manifest.Verified)
	fmt.Println()

	// Step 5: prompt if unverified
	if !manifest.Verified {
		fmt.Printf("  ⚠  %s is unverified. Install anyway? (y/n): ", name)
		var confirm string
		fmt.Scan(&confirm)
		if confirm != "y" {
			fmt.Println("  aborted.")
			return nil
		}
	}

	// Step 6: build the module binary
	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(BinDir(), binName)

	fmt.Printf("  building %s@%s...\n", name, version)
	if out, err := exec.Command("go", "build", "-trimpath", "-o", binPath, srcDir).CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %s: %w", string(out), err)
	}

	// Step 7: register locally
	reg := LoadRegistry()
	reg[name] = RegistryEntry{
		BinPath:  binPath,
		Version:  manifest.Version,
		Author:   manifest.Author,
		Official: manifest.Official,
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

// BinPath returns the binary path for an installed module.
func BinPath(name string) (string, error) {
	reg := LoadRegistry()
	entry, ok := reg[name]
	if !ok {
		return "", fmt.Errorf("module %q is not installed", name)
	}
	return entry.BinPath, nil
}

// --- helpers

// latestVersion returns the highest semver directory name inside moduleDir.
// For now picks the last entry alphabetically — good enough for semver folders.
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
	// last alphabetically = highest semver (0.1.0 < 0.2.0 < 1.0.0)
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