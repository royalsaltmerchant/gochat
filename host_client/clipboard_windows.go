package main

import (
	"io"
	"os/exec"
	"syscall"
)

func Copy(text string) error {
	cmd := exec.Command("clip")

	// 🧙 Hide the terminal window
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

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
