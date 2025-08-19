package execer

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"time"
)

// RunShell runs the given command string using the platform shell.
// - Windows: cmd /C "<cmd>"
// - Others : /bin/sh -c "<cmd>"
// Environment variables from env map are injected.
func RunShell(ctx context.Context, command string, env map[string]string) error {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		c = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	// attach env
	if len(env) > 0 {
		en := make([]string, 0, len(env))
		for k, v := range env {
			en = append(en, k+"="+v)
		}
		c.Env = append(c.Env, en...)
	}
	// run
	if err := c.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	select {
	case <-ctx.Done():
		_ = c.Process.Kill()
		return errors.New("timeout/canceled")
	case err := <-done:
		return err
	case <-time.After(24 * time.Hour):
		_ = c.Process.Kill()
		return errors.New("guard-timeout")
	}
}
