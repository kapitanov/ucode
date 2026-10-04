package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kapitanov/ucode/internal/etc/difftool"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/sandbox/guardrails"
	"github.com/kapitanov/ucode/internal/tui"
)

func Direct(ui iface.UI) iface.Sandbox {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Resolve symlinks in the working directory so subsequent comparisons are
	// made against the physical path. This avoids false rejections when wd is
	// itself reached through a symlink.
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		panic(err)
	}

	tui.Printf("%% Running in direct mode. Working directory: %q", wd)

	s := &directSandbox{
		wd: wd,
		ui: ui,
	}
	s.validateDependencies()

	return s
}

type directSandbox struct {
	wd string
	ui iface.UI
}

func (*directSandbox) RequireManualValidation() bool { return false }

func (s *directSandbox) ReadFile(path string) ([]byte, error) {
	path, err := guardrails.NormalizePath(s.wd, path)
	if err != nil {
		return nil, err
	}
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	}

	return data, nil
}

func (s *directSandbox) ListFiles(dir string) (dirs []string, files []string, err error) {
	dir, err = guardrails.NormalizePath(s.wd, dir)
	if err != nil {
		return nil, nil, doublestar.ErrPatternNotExist
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directory %q: %v", dir, err)
	}

	for _, entry := range entries {
		if !guardrails.IsAllowedPath(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		} else {
			files = append(files, entry.Name())
		}
	}

	return
}

func (s *directSandbox) SearchFiles(pattern string, path, fileType *string, caseSensitive *bool) (results []string, err error) {
	if path == nil || *path == "" {
		path = new(".")
	}

	effectivePath, err := guardrails.NormalizePath(s.wd, *path)
	if err != nil {
		return nil, err
	}
	if !guardrails.IsAllowedPath(effectivePath) {
		return nil, fmt.Errorf("access to path %q is not allowed", effectivePath)
	}

	// Build ripgrep command
	rgArgs := []string{"--line-number", "--with-filename", "--color=never"}

	// Add case sensitivity flag
	if caseSensitive == nil || !*caseSensitive {
		rgArgs = append(rgArgs, "--ignore-case")
	}

	// Add file type filter if specified
	if fileType != nil && *fileType != "" {
		rgArgs = append(rgArgs, "--type", *fileType)
	}

	rgArgs = append(rgArgs, pattern, effectivePath)

	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.Output()

	// ripgrep returns exit code 1 when no matches are found, which is not an error
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("search failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	results = strings.Split(outputStr, "\n")
	return
}

func (s *directSandbox) WriteFile(path string, bs []byte) (err error) {
	path, err = guardrails.NormalizePath(s.wd, path)
	if err != nil {
		return err
	}
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %v", dir, err)
	}

	err = os.WriteFile(path, bs, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %q: %v", path, err)
	}

	return nil
}

func (s *directSandbox) PatchFile(path, diff string) (bs []byte, err error) {
	path, err = guardrails.NormalizePath(s.wd, path)
	if err != nil {
		return nil, err
	}
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %q: %v", dir, err)
	}

	original := ""
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	} else if err == nil {
		original = string(existing)
	}

	patched, err := difftool.Apply(original, diff)
	if err != nil {
		return nil, fmt.Errorf("failed to apply diff to %q: %v", path, err)
	}

	err = os.WriteFile(path, []byte(patched), 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to write file %q: %v", path, err)
	}

	return []byte(patched), nil
}

func (s *directSandbox) RemoveFile(path string) (err error) {
	path, err = guardrails.NormalizePath(s.wd, path)
	if err != nil {
		return err
	}
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove file %q: %v", path, err)
	}

	return nil
}

func (s *directSandbox) ShellCommand(command []string) (output string, exitCode int, err error) {
	args := []string{"-c"}
	args = append(args, command...)
	cmd := exec.Command("sh", args...)
	var rawOutput []byte
	rawOutput, err = cmd.CombinedOutput()
	output = string(rawOutput)
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
			err = nil
			return
		}

		err = fmt.Errorf("command failed with error: %s\nOutput: %s", err.Error(), output)
		return
	}

	return
}

func (*directSandbox) Close() {}

func (*directSandbox) validateDependencies() {
	if _, err := exec.LookPath("rg"); err != nil {
		panic("ripgrep (rg) is not installed or not in PATH")
	}
}
