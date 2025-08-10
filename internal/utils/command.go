package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %s, stderr: %s", err, stderr.String())
	}
	return out.String(), nil
}

func RunBashCommand(cmd string, verbose bool) error {
	fmt.Printf("⚡ Running: %s\n", cmd)
	command := exec.Command("bash", "-c", cmd)
	if verbose {
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
	} else {
		command.Stdout = io.Discard
		command.Stderr = io.Discard
	}
	return command.Run()
}
