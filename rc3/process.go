package rc3

// ProcessState holds the process OCI specs along with various fields
// required by containerd
type ProcessState struct {
	Process
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
