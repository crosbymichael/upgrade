package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/crosbymichael/upgrade/rc3"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	ConfigFilename = "config.json"
)

// UpgradeSpec upgrades a spec from the previous version to the current spec version
func UpgradeSpec(dir string) error {
	old, err := readRc3(dir)
	if err != nil {
		return err
	}
	spec, err := mapSpec(old)
	if err != nil {
		return err
	}
	return rewriteJSON(filepath.Join(dir, ConfigFilename), spec)
}

func readRc3(dir string) (*rc3.Spec, error) {
	f, err := os.Open(filepath.Join(dir, ConfigFilename))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var spec rc3.Spec
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func mapSpec(old *rc3.Spec) (*specs.Spec, error) {
	var spec specs.Spec
	spec.Version = specs.Version
	spec.Annotations = old.Annotations
	for _, h := range old.Hooks.Prestart {
		spec.Hooks.Prestart = append(spec.Hooks.Prestart, mapHook(h))
	}
	for _, h := range old.Hooks.Poststart {
		spec.Hooks.Poststart = append(spec.Hooks.Poststart, mapHook(h))
	}
	for _, h := range old.Hooks.Poststop {
		spec.Hooks.Poststop = append(spec.Hooks.Poststop, mapHook(h))
	}
	spec.Hostname = old.Hostname
	for _, m := range old.Mounts {
		spec.Mounts = append(spec.Mounts, specs.Mount{
			Destination: m.Destination,
			Options:     m.Options,
			Source:      m.Source,
			Type:        m.Type,
		})
	}
	spec.Root = &specs.Root{
		Path:     old.Root.Path,
		Readonly: old.Root.Readonly,
	}
	spec.Process = &specs.Process{
		Terminal:        old.Process.Terminal,
		Args:            old.Process.Args,
		Env:             old.Process.Env,
		Cwd:             old.Process.Cwd,
		SelinuxLabel:    old.Process.SelinuxLabel,
		ApparmorProfile: old.Process.ApparmorProfile,
		// OOMScoreAdj: old.Process.o
		Capabilities: &specs.LinuxCapabilities{
			Bounding:    old.Process.Capabilities,
			Effective:   old.Process.Capabilities,
			Inheritable: old.Process.Capabilities,
			Permitted:   old.Process.Capabilities,
		},
		NoNewPrivileges: old.Process.NoNewPrivileges,
	}
	for _, r := range old.Process.Rlimits {
		spec.Process.Rlimits = append(spec.Process.Rlimits, specs.LinuxRlimit{
			Type: r.Type,
			Hard: r.Hard,
			Soft: r.Soft,
		})
	}
	if old.Process.ConsoleSize.Height != 0 || old.Process.ConsoleSize.Width != 0 {
		spec.Process.ConsoleSize = &specs.Box{
			Width:  old.Process.ConsoleSize.Width,
			Height: old.Process.ConsoleSize.Height,
		}
	}
	if old.Linux != nil {
		if old.Linux.Resources != nil {
			spec.Process.OOMScoreAdj = old.Linux.Resources.OOMScoreAdj
		}
	}
	if old.Windows != nil {

	}
	if old.Solaris != nil {

	}

	return &spec, nil
}

func mapHook(h rc3.Hook) specs.Hook {
	return specs.Hook{
		Path:    h.Path,
		Args:    h.Args,
		Env:     h.Env,
		Timeout: h.Timeout,
	}
}

func rewriteJSON(path string, v interface{}) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}
