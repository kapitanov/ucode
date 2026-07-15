package sandbox

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kapitanov/ucode/internal/etc/difftool"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/sandbox/guardrails"
	"github.com/kapitanov/ucode/internal/tui"
)

func Isolated() iface.Sandbox {
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

	tui.Printf("%% Running in isolated mode. Working directory: %q", wd)

	s := &isolatedSandbox{
		wd: wd,
	}
	s.validateDependencies()
	s.buildContainer()

	return s
}

type isolatedSandbox struct {
	wd             string
	containerImage string
}

func (*isolatedSandbox) RequireManualValidation() bool { return false }

func (s *isolatedSandbox) ReadFile(path string) ([]byte, error) {
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	bs, err := s.mustExecf("cat %q", path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	}

	return bs, nil
}

func (s *isolatedSandbox) ListFiles(dir string) (dirs []string, files []string, err error) {
	if !guardrails.IsAllowedPath(dir) {
		return nil, nil, fmt.Errorf("access to path %q is not allowed", dir)
	}

	dirs, err = s.listFilesHelper(dir, "d")
	if err != nil {
		return nil, nil, err
	}

	files, err = s.listFilesHelper(dir, "f")
	if err != nil {
		return nil, nil, err
	}

	return
}

func (s *isolatedSandbox) listFilesHelper(dir, typeFilter string) ([]string, error) {
	// find . -maxdepth 1 -not -name '.' -type f | xargs realpath | sort
	// find . -maxdepth 1 -not -name '.' -type d | xargs realpath | sort
	rawOutput, err := s.mustExecf("find %q -maxdepth 1 -not -name '.' -type %s | xargs realpath | sort", dir, typeFilter)
	if err != nil {
		return nil, err
	}

	var entries []string
	for _, line := range strings.Split(string(rawOutput), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !guardrails.IsAllowedPath(line) {
			continue
		}
		entries = append(entries, line)
	}

	return entries, nil
}

func (s *isolatedSandbox) SearchFiles(pattern string, path, fileType *string, caseSensitive *bool) (results []string, err error) {
	effectivePath := "."
	if path != nil && *path != "" {
		effectivePath = *path
	}

	if !guardrails.IsAllowedPath(effectivePath) {
		return nil, fmt.Errorf("access to path %q is not allowed", effectivePath)
	}

	// Build ripgrep command
	command := "rg --line-number --with-filename --color=never"

	// Add case sensitivity flag
	if caseSensitive == nil || !*caseSensitive {
		command += " --ignore-case"
	}

	// Add file type filter if specified
	if fileType != nil && *fileType != "" {
		command += " --type "
		command += *fileType
	}

	command += fmt.Sprintf(" %q %q", pattern, effectivePath)

	output, exitCode, err := s.exec(command)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// ripgrep returns exit code 1 when no matches are found, which is not an error
	if exitCode == 1 {
		return nil, nil
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("search failed with exit code %d:\n%s", exitCode, output)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	results = strings.Split(outputStr, "\n")
	return
}

func (s *isolatedSandbox) WriteFile(path string, bs []byte) (err error) {
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	dir := filepath.Dir(path)
	_, err = s.mustExecf("mkdir -p %q && cat > %q <<EOF\n%s\nEOF", dir, path, string(bs))
	if err != nil {
		return fmt.Errorf("failed to write file %q: %v", path, err)
	}

	return nil
}

func (s *isolatedSandbox) PatchFile(path, diff string) (bs []byte, err error) {
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	original, err := s.mustExecf("cat %q || true", path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	}

	patched, err := difftool.Apply(string(original), diff)
	if err != nil {
		return nil, fmt.Errorf("failed to apply diff to %q: %v", path, err)
	}

	err = s.WriteFile(path, []byte(patched))
	return []byte(patched), err
}

func (s *isolatedSandbox) RemoveFile(path string) (err error) {
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	_, err = s.mustExecf("rm -rf %q", path)
	if err != nil {
		return fmt.Errorf("failed to remove file %q: %v", path, err)
	}

	return nil
}

func (s *isolatedSandbox) ShellCommand(command string) (output string, exitCode int, err error) {
	var rawOutput []byte
	rawOutput, exitCode, err = s.exec(command)
	output = string(rawOutput)
	return
}

func (*isolatedSandbox) validateDependencies() {
	if _, err := exec.LookPath("docker"); err != nil {
		panic("docker is not installed or not in PATH")
	}
}

func (s *isolatedSandbox) buildContainer() {
	tui.Printf("%% Buiding sandbox docker container:")

	dockerfile, err := os.CreateTemp("", "")
	if err != nil {
		panic(fmt.Errorf("failed to create temporary Dockerfile: %v", err))
	}
	_, err = dockerfile.Write(dockerfileContent)
	if err != nil {
		panic(fmt.Errorf("failed to write to temporary Dockerfile: %v", err))
	}
	_ = dockerfile.Close()

	tui.Printf("%% docker build -q -f %s . …", dockerfile.Name())
	cmd := exec.Command("docker", "build", "-q", "-f", dockerfile.Name(), ".")

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("failed to build Docker image: %v\nOutput: %s", err, output))
	}

	s.containerImage = strings.TrimSpace(string(output))

	tui.Printf("%% → %q\n", s.containerImage)
}

func (s *isolatedSandbox) exec(command string) ([]byte, int, error) {
	cmd := exec.Command("docker", "run", "-t", "--rm", "-v", fmt.Sprintf("%s:/mnt", s.wd), "-w", "/mnt", s.containerImage, "bash", "-c", command)

	// tui.Printf("%% docker run -t --rm -v %s:/mnt -w /mnt %s bash -c %q\n", s.wd, s.containerImage, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return output, exitErr.ExitCode(), nil
		}
		return nil, 0, fmt.Errorf("failed to execute command: %v\nOutput: %s", err, output)
	}

	return output, 0, nil
}

func (s *isolatedSandbox) mustExecf(format string, args ...any) ([]byte, error) {
	return s.mustExec(fmt.Sprintf(format, args...))
}

func (s *isolatedSandbox) mustExec(command string) ([]byte, error) {
	cmd := exec.Command("docker", "run", "-t", "--rm", "-v", fmt.Sprintf("%s:/mnt", s.wd), "-w", "/mnt", s.containerImage, "sh", "-c", command)

	// tui.Printf("%% docker run -t --rm -v %s:/mnt -w /mnt %s bash -c %q\n", s.wd, s.containerImage, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %v\nOutput: %s", err, output)
	}

	return output, nil
}

var (
	//go:embed Dockerfile
	dockerfileContent []byte
)
