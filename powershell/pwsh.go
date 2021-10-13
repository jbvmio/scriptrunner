package powershell

import (
	"bytes"
	"os/exec"
)

// PowerShell struct
type PowerShell struct {
	powerShell string
	workDir    string
}

// New create new session
func New(workDir string) *PowerShell {
	ps, _ := exec.LookPath("powershell.exe")
	return &PowerShell{
		powerShell: ps,
		workDir:    workDir,
	}
}

// Execute runs the given command arguments using Powershell.
func (p *PowerShell) Execute(args ...string) (stdOut string, stdErr string, err error) {
	args = append([]string{"-NoProfile", "-NonInteractive"}, args...)
	cmd := exec.Command(p.powerShell, args...)
	cmd.Dir = p.workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stdOut, stdErr = stdout.String(), stderr.String()
	return
}
