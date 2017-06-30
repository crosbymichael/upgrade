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

// ProcessState holds the process OCI specs along with various fields
// required by containerd
type ProcessState struct {
	specs.Process
	Exec        bool     `json:"exec"`
	Stdin       string   `json:"containerdStdin"`
	Stdout      string   `json:"containerdStdout"`
	Stderr      string   `json:"containerdStderr"`
	RuntimeArgs []string `json:"runtimeArgs"`
	NoPivotRoot bool     `json:"noPivotRoot"`

	Checkpoint string `json:"checkpoint"`
	RootUID    int    `json:"rootUID"`
	RootGID    int    `json:"rootGID"`
}

// UpgradeSpec upgrades a spec from the previous version to the current spec version
func UpgradeSpec(dir string) error {
	configPath := filepath.Join(dir, ConfigFilename)
	old, err := readRc3(configPath)
	if err != nil {
		return err
	}
	spec, err := mapSpec(old)
	if err != nil {
		return err
	}
	return rewriteJSON(configPath, spec)
}

func readRc3(configPath string) (*rc3.Spec, error) {
	f, err := os.Open(configPath)
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
	spec.Hooks = new(specs.Hooks)
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
	spec.Process = mapProcess(old.Process)
	if old.Linux != nil {
		spec.Linux = mapLinux(old.Linux, spec.Process)
	}
	if old.Windows != nil {
		// we don't need to map windows because it does not use runc or containerd
	}
	if old.Solaris != nil {
		// we don't do solaris anymore
	}
	return &spec, nil
}

func mapProcessState(old rc3.ProcessState) ProcessState {
	return ProcessState{
		Process:     *mapProcess(old.Process),
		Exec:        old.Exec,
		Stdin:       old.Stdin,
		Stdout:      old.Stdout,
		Stderr:      old.Stderr,
		RuntimeArgs: old.RuntimeArgs,
		NoPivotRoot: old.NoPivotRoot,

		Checkpoint: old.Checkpoint,
		RootUID:    old.RootUID,
		RootGID:    old.RootGID,
	}
}

func mapProcess(old rc3.Process) *specs.Process {
	process := &specs.Process{
		Terminal:        old.Terminal,
		Args:            old.Args,
		Env:             old.Env,
		Cwd:             old.Cwd,
		SelinuxLabel:    old.SelinuxLabel,
		ApparmorProfile: old.ApparmorProfile,
		Capabilities: &specs.LinuxCapabilities{
			Bounding:    old.Capabilities,
			Effective:   old.Capabilities,
			Inheritable: old.Capabilities,
			Permitted:   old.Capabilities,
		},
		NoNewPrivileges: old.NoNewPrivileges,
	}
	for _, r := range old.Rlimits {
		process.Rlimits = append(process.Rlimits, specs.LinuxRlimit{
			Type: r.Type,
			Hard: r.Hard,
			Soft: r.Soft,
		})
	}
	if old.ConsoleSize.Height != 0 || old.ConsoleSize.Width != 0 {
		process.ConsoleSize = &specs.Box{
			Width:  old.ConsoleSize.Width,
			Height: old.ConsoleSize.Height,
		}
	}
	process.User = specs.User{
		UID:            old.User.UID,
		GID:            old.User.GID,
		Username:       old.User.Username,
		AdditionalGids: old.User.AdditionalGids,
	}
	return process
}

func mapHook(h rc3.Hook) specs.Hook {
	return specs.Hook{
		Path:    h.Path,
		Args:    h.Args,
		Env:     h.Env,
		Timeout: h.Timeout,
	}
}

func mapLinux(old *rc3.Linux, process *specs.Process) *specs.Linux {
	linux := &specs.Linux{
		Sysctl:            old.Sysctl,
		MountLabel:        old.MountLabel,
		MaskedPaths:       old.MaskedPaths,
		ReadonlyPaths:     old.ReadonlyPaths,
		RootfsPropagation: old.RootfsPropagation,
	}
	if old.CgroupsPath != nil {
		linux.CgroupsPath = *old.CgroupsPath
	}
	for _, d := range old.Devices {
		linux.Devices = append(linux.Devices, specs.LinuxDevice{
			Path:     d.Path,
			Type:     d.Type,
			Major:    d.Major,
			Minor:    d.Minor,
			FileMode: d.FileMode,
			UID:      d.UID,
			GID:      d.GID,
		})
	}
	for _, i := range old.UIDMappings {
		linux.UIDMappings = append(linux.UIDMappings, mapID(i))
	}
	for _, i := range old.GIDMappings {
		linux.GIDMappings = append(linux.GIDMappings, mapID(i))
	}
	for _, n := range old.Namespaces {
		linux.Namespaces = append(linux.Namespaces, specs.LinuxNamespace{
			Type: specs.LinuxNamespaceType(n.Type),
			Path: n.Path,
		})
	}
	if old.Seccomp != nil {
		linux.Seccomp = mapSeccomp(old.Seccomp)
	}
	if old.Resources != nil {
		linux.Resources = mapResources(old.Resources, process)
	}
	// no need to map IntelRdt
	return linux
}

func mapSeccomp(old *rc3.Seccomp) *specs.LinuxSeccomp {
	var s specs.LinuxSeccomp
	s.DefaultAction = specs.LinuxSeccompAction(old.DefaultAction)
	for _, a := range old.Architectures {
		s.Architectures = append(s.Architectures, specs.Arch(a))
	}
	for _, a := range old.Syscalls {
		sys := specs.LinuxSyscall{
			Names:  []string{a.Name},
			Action: specs.LinuxSeccompAction(a.Action),
		}
		for _, arg := range a.Args {
			sys.Args = append(sys.Args, specs.LinuxSeccompArg{
				Index:    arg.Index,
				Value:    arg.Value,
				ValueTwo: arg.ValueTwo,
				Op:       specs.LinuxSeccompOperator(arg.Op),
			})
		}
		s.Syscalls = append(s.Syscalls, sys)
	}
	return &s
}

func mapID(old rc3.IDMapping) specs.LinuxIDMapping {
	return specs.LinuxIDMapping{
		HostID:      old.HostID,
		ContainerID: old.ContainerID,
		Size:        old.Size,
	}
}

func defaultString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toInt(i *uint64) *int64 {
	if i == nil {
		return nil
	}
	ii := *i
	iv := int64(ii)
	return &iv
}

func mapResources(old *rc3.Resources, process *specs.Process) *specs.LinuxResources {
	r := &specs.LinuxResources{}
	// this field moved from resources to the process
	process.OOMScoreAdj = old.OOMScoreAdj

	r.DisableOOMKiller = old.DisableOOMKiller
	for _, d := range old.Devices {
		r.Devices = append(r.Devices, specs.LinuxDeviceCgroup{
			Allow:  d.Allow,
			Type:   defaultString(d.Type),
			Major:  d.Major,
			Minor:  d.Minor,
			Access: defaultString(d.Access),
		})
	}
	if old.Memory != nil {
		m := old.Memory
		r.Memory = &specs.LinuxMemory{
			Limit:       toInt(m.Limit),
			Reservation: toInt(m.Reservation),
			Swap:        toInt(m.Swap),
			Kernel:      toInt(m.Kernel),
			KernelTCP:   toInt(m.KernelTCP),
			// TODO: fix -1 here
			Swappiness: m.Swappiness,
		}
	}
	if old.CPU != nil {
		c := old.CPU
		r.CPU = &specs.LinuxCPU{
			Shares:          c.Shares,
			Quota:           toInt(c.Quota),
			Period:          c.Period,
			RealtimeRuntime: toInt(c.RealtimeRuntime),
			RealtimePeriod:  c.RealtimePeriod,
			Cpus:            defaultString(c.Cpus),
			Mems:            defaultString(c.Mems),
		}
	}
	if old.Pids != nil {
		var (
			p  = old.Pids
			nv int64
		)
		if p.Limit != nil {
			nv = *p.Limit
		}
		r.Pids = &specs.LinuxPids{
			Limit: nv,
		}
	}
	if old.BlockIO != nil {
		b := old.BlockIO
		r.BlockIO = &specs.LinuxBlockIO{
			Weight:                  b.Weight,
			LeafWeight:              b.LeafWeight,
			WeightDevice:            mapBlkioW(b.WeightDevice),
			ThrottleReadBpsDevice:   mapBlkioT(b.ThrottleReadBpsDevice),
			ThrottleWriteBpsDevice:  mapBlkioT(b.ThrottleWriteBpsDevice),
			ThrottleReadIOPSDevice:  mapBlkioT(b.ThrottleReadIOPSDevice),
			ThrottleWriteIOPSDevice: mapBlkioT(b.ThrottleWriteIOPSDevice),
		}
	}
	for _, h := range old.HugepageLimits {
		var nv uint64
		if h.Limit != nil {
			nv = *h.Limit
		}
		r.HugepageLimits = append(r.HugepageLimits, specs.LinuxHugepageLimit{
			Pagesize: defaultString(h.Pagesize),
			Limit:    nv,
		})
	}
	if old.Network != nil {
		n := old.Network
		r.Network = &specs.LinuxNetwork{
			ClassID:    n.ClassID,
			Priorities: mapNetprio(n.Priorities),
		}
	}
	return r
}

func mapNetprio(old []rc3.InterfacePriority) (out []specs.LinuxInterfacePriority) {
	for _, n := range old {
		out = append(out, specs.LinuxInterfacePriority{
			Name:     n.Name,
			Priority: n.Priority,
		})
	}
	return out
}

func mapBlkioW(old []rc3.WeightDevice) (out []specs.LinuxWeightDevice) {
	for _, o := range old {
		n := specs.LinuxWeightDevice{}
		n.Major = o.Major
		n.Minor = o.Minor
		n.Weight = o.Weight
		n.LeafWeight = o.LeafWeight
		out = append(out, n)
	}
	return out
}

func mapBlkioT(old []rc3.ThrottleDevice) (out []specs.LinuxThrottleDevice) {
	for _, o := range old {
		n := specs.LinuxThrottleDevice{}
		n.Major = o.Major
		n.Minor = o.Minor
		var nv uint64
		if o.Rate != nil {
			nv = *o.Rate
		}
		n.Rate = nv
		out = append(out, n)
	}
	return out
}

func rewriteJSON(path string, v interface{}) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}
