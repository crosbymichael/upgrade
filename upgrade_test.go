package upgrade

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/docker/go/canonical/json"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// TODO: generate patches with jq -S and diff
// Check in the patches.
// Apply those reverse patches and compare with original json

var defaultCapsList = []string{
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FSETID",
	"CAP_FOWNER",
	"CAP_MKNOD",
	"CAP_NET_RAW",
	"CAP_SETGID",
	"CAP_SETUID",
	"CAP_SETFCAP",
	"CAP_SETPCAP",
	"CAP_NET_BIND_SERVICE",
	"CAP_SYS_CHROOT",
	"CAP_KILL",
	"CAP_AUDIT_WRITE",
}

var defaultCaps = linuxCapabilities{&specs.LinuxCapabilities{
	Bounding:    defaultCapsList,
	Effective:   defaultCapsList,
	Inheritable: defaultCapsList,
	Permitted:   defaultCapsList,
}}

func decode(t *testing.T, filename string, x interface{}) (content []byte) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("readfile %s: (%T) %v", filename, err, err)
	}
	if err := json.Unmarshal(content, x); err != nil {
		t.Fatalf("decode %s to %T: (%T) %v", filename, x, err, err)
	}
	return content
}

func TestConfig(t *testing.T) {
	for _, d := range [...]struct {
		filename         string
		caps             linuxCapabilities
		memorySwappiness memorySwappiness
	}{
		{"testfiles/config-sorted-17.03.json", defaultCaps, memorySwappiness{nil}},
		{"testfiles/config-unwrapped-17.05.json", defaultCaps, memorySwappiness{nil}},
		{"testfiles/config-unwrapped-17.06.0.json", defaultCaps, memorySwappiness{nil}},
		{"testfiles/config-unwrapped-17.06.1.json", defaultCaps, memorySwappiness{nil}},
		// TODO: add tests for non-nil memorySwappiness
	} {
		var spec Spec
		content := decode(t, d.filename, &spec)

		if !reflect.DeepEqual(spec.Process.Capabilities.V, d.caps.V) {
			t.Fatalf("validate %s (capabilities): %#v | %#v", d.filename, d.caps.V, spec.Process.Capabilities.V)
		}
		if d.memorySwappiness.compare(spec.Linux.Resources.Memory.Swappiness) != 0 {
			t.Fatalf("validate %s (memorySwappiness): %s | %s", d.filename, d.memorySwappiness.V, spec.Linux.Resources.Memory.Swappiness)
		}

		marshalCompare(t, d.filename, content, &spec)
	}

}

func TestProcess(t *testing.T) {
	for _, d := range [...]struct {
		filename string
		caps     linuxCapabilities
	}{
		{"testfiles/process-unwrapped-17.03.json", defaultCaps},
		{"testfiles/process-unwrapped-17.05.json", defaultCaps},
		{"testfiles/process-unwrapped-17.06.0.json", defaultCaps},
		{"testfiles/process-unwrapped-17.06.1.json", defaultCaps},
	} {
		var s ProcessState
		decode(t, d.filename, &s)

		if !reflect.DeepEqual(s.Capabilities.V, d.caps.V) {
			t.Fatalf("validate %s (capabilities): %#v | %#v", d.filename, d.caps.V, s.Capabilities.V)
		}
	}
}

func TestState(t *testing.T) {
	for _, d := range [...]struct {
		filename             string
		initProcessStartTime initProcessStartTimeType
		memorySwappiness     memorySwappiness
	}{
		{"testfiles/state-unwrapped-17.03.json", 8468990, memorySwappiness{nil}},
		{"testfiles/state-unwrapped-17.05.json", 8504596, memorySwappiness{nil}},
		{"testfiles/state-unwrapped-17.06.0.json", 8497004, memorySwappiness{nil}},
		{"testfiles/state-unwrapped-17.06.1.json", 8497004, memorySwappiness{nil}},
		// TODO: add tests for non-nil memorySwappiness
	} {
		var s State
		decode(t, d.filename, &s)

		if s.InitProcessStartTime != d.initProcessStartTime {
			t.Fatalf("validate %s (initProcessStartTime): %d | %d", d.filename, d.initProcessStartTime, s.InitProcessStartTime)
		}

		if d.memorySwappiness.compare(s.Config.Cgroups.Resources.MemorySwappiness) != 0 {
			t.Fatalf("validate %s (memorySwappiness): %s | %s", d.filename, d.memorySwappiness.V, s.Config.Cgroups.Resources.MemorySwappiness)
		}
	}
}

const debug = false

func marshalCompare(t *testing.T, filename string, content []byte, x interface{}) {
	marshalled, err := json.MarshalCanonical(x)
	if err != nil {
		t.Fatalf("marshal %s (%T): (%T) %v", filename, x, err, err)
	}
	if bytes.Compare(content, marshalled) != 0 {
		// for display purposes (easier diff), indent the json.
		marshalled, err := indentBytes(marshalled, "", "\t")
		if err != nil {
			t.Fatalf("validate %s (marshal+indent1): %v", filename, err)
		}
		content, err := indentBytes(content, "", "\t")
		if err != nil {
			t.Fatalf("validate %s (marshal+indent2): %v", filename, err)
		}
		t.Fatalf("validate %s (marshal):\n%s\n==================================\n%s\n", filename, content, marshalled)
	}
}
func indentBytes(b []byte, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, prefix, indent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
