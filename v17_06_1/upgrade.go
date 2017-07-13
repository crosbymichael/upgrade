package v17_06_1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type file struct {
	*os.File
	name string
	x    interface{}
	buf  bytes.Buffer
}

func Upgrade(runcState, containerdConfig, containerdProcess string) error {
	files := []*file{
		&file{name: runcState, x: new(State)},
		&file{name: containerdConfig, x: new(Spec)},
		&file{name: containerdProcess, x: new(ProcessState)},
	}
	// error out if any of the files have issues being decoded
	// before overwriting them, to prevent being in a mixed state.
	for _, f := range files {
		fi, err := os.Stat(f.name)
		if err != nil {
			return err
		}
		f.File, err = os.OpenFile(f.name, os.O_RDWR, fi.Mode())
		if err != nil {
			return err
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(f.x); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
	}
	for _, f := range files {
		// error out if any of the files have issues being encoded
		// before overwriting them, to prevent being in a mixed state.
		if err := json.NewEncoder(&f.buf).Encode(f.x); err != nil {
			return err
		}
	}
	var errs []string
	for _, f := range files {
		if _, err := f.Write(f.buf.Bytes()); err != nil {
			errs = append(errs, fmt.Sprintf("error writing to %s: %v", f.name, err))
		}
	}
	if errs != nil {
		return fmt.Errorf(strings.Join(errs, ", "))
	}
	return nil
}
