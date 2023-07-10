package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run a command in the user's environment
func Shell(cmd string, arg ...string) error {
	fmt.Println(cmd, strings.Join(arg, " "))
	exe := exec.Command(cmd, arg...)
	exe.Env = os.Environ()
	exe.Stderr = os.Stderr
	exe.Stdout = os.Stdout
	exe.Stdin = os.Stdin

	if err := exe.Run(); err != nil {
		return err
	}

	return nil
}

// Run a shell command prefixed by sudo, return error
func Sudo(cmd string, arg ...string) error {
	c := []string{cmd}
	c = append(c, arg...)

	return Shell("sudo", c...)
}

// Run a command in the user's environment capturing the output
func Capture(cmd string, arg ...string) ([]byte, error) {
	exe := exec.Command(cmd, arg...)
	exe.Env = os.Environ()

	out, err := exe.CombinedOutput()
	return out, err
}
