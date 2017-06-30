package upgrade

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/crosbymichael/upgrade/rc3"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const debug = false

func TestConfig(t *testing.T) {
	old, err := readRc3(filepath.Join("testfiles", "config.json-17.03"))
	if err != nil {
		t.Fatal(err)
	}
	new, err := mapSpec(old)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testfiles/config.json-17.06.1")
	if err != nil {
		t.Fatal(err)
	}
	var s specs.Spec
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		t.Fatal(err)
	}
	s.Annotations = nil
	new.Annotations = nil

	if err := compare(s, new); err != nil {
		t.Fatal(err)
	}
}

func compare(old, new interface{}) error {
	bRemapped, err := json.Marshal(new)
	if err != nil {
		return err
	}
	bCorrect, err := json.Marshal(old)
	if err != nil {
		return err
	}
	if debug {
		fmt.Println(string(bRemapped))
		fmt.Println()
		fmt.Println(string(bCorrect))
		fmt.Println()
		fmt.Println()
	}
	if bytes.Compare(bRemapped, bCorrect) != 0 {
		return errors.New("remapping of rc3 spec did not match new config file")
	}
	return nil
}

func TestState(t *testing.T) {
}

func TestProcess(t *testing.T) {
	f, err := os.Open("testfiles/process.json-17.03")
	if err != nil {
		t.Fatal(err)
	}
	var old rc3.ProcessState
	if err := json.NewDecoder(f).Decode(&old); err != nil {
		t.Fatal(err)
	}
	new := mapProcessState(old)
	f, err = os.Open("testfiles/process.json-17.06.1")
	if err != nil {
		t.Fatal(err)
	}
	var s ProcessState
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if err := compare(s, new); err != nil {
		t.Fatal(err)
	}
}
