package v17_06_1

import (
	"encoding/json"
	"os"
)

func Upgrade(runcState, containerdConfig, containerdProcess string) error {
	if err := UpgradeState(runcState); err != nil {
		return err
	}
	if err := UpgradeConfig(containerdConfig); err != nil {
		return err
	}
	return UpgradeProcessState(containerdProcess)
}

func UpgradeState(filename string) error {
	var x State
	return remarshal(filename, &x)
}

func UpgradeConfig(filename string) error {
	var x Spec
	return remarshal(filename, &x)
}

func UpgradeProcessState(filename string) error {
	var x ProcessState
	return remarshal(filename, &x)
}

func remarshal(filename string, x interface{}) error {
	fi, err := os.Stat(filename)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_RDWR, fi.Mode())
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(x); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(x)
}
