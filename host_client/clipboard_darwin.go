//go:build darwin && !headless

package main

import (
	"io"
	"os/exec"
)

func Copy(text string) error {
	cmd := exec.Command("pbcopy")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_, err = io.WriteString(in, text)
	if err != nil {
		return err
	}
	in.Close()
	return cmd.Wait()
}
