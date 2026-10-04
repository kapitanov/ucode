package sandbox

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kapitanov/ucode/internal/etc/difftool"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/sandbox/guardrails"
	"github.com/kapitanov/ucode/internal/tui"
)

// The isolated sandbox runs a single long-lived container per process and talks to it over a
// newline-delimited JSON protocol on its stdin/stdout, rather than spawning a fresh `docker run`
// for every file operation. The container's side of the protocol is implemented by the small Go
// program in ./containerfs/main.go (built into the image by ./containerfs/Dockerfile); see
// sandbox_isolated_container.go for the matching request/response types and the code that
// manages the container's lifecycle.
//
// Resilience: a failed call is retried (with a limit) by restarting the container - see
// containerAgent.call. Containers are labeled with their owning process's PID; on startup any
// container whose owning process is no longer alive is treated as orphaned and removed.

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

	validateDockerDependency()

	containerImage, err := buildContainer()
	if err != nil {
		panic(err)
	}

	return &isolatedSandbox{
		agent: newContainerAgent(wd, containerImage),
	}
}

type isolatedSandbox struct {
	agent *containerAgent
}

func (*isolatedSandbox) RequireManualValidation() bool { return false }

func (s *isolatedSandbox) ReadFile(path string) ([]byte, error) {
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	resp, err := s.agent.call(containerRequest{ReadFile: &readFileRequest{Path: path}})
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("failed to read file %q: %s", path, resp.Error)
	}

	bs, err := base64Decode(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode contents of %q: %w", path, err)
	}

	return bs, nil
}

func (s *isolatedSandbox) ListFiles(dir string) (dirs []string, files []string, err error) {
	if !guardrails.IsAllowedPath(dir) {
		return nil, nil, fmt.Errorf("access to path %q is not allowed", dir)
	}

	resp, err := s.agent.call(containerRequest{ListFiles: &listFilesRequest{Dir: dir}})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list files in %q: %w", dir, err)
	}
	if resp.Error != "" {
		return nil, nil, fmt.Errorf("failed to list files in %q: %s", dir, resp.Error)
	}

	return filterAllowedPaths(resp.Dirs), filterAllowedPaths(resp.Files), nil
}

func filterAllowedPaths(paths []string) []string {
	var allowed []string
	for _, p := range paths {
		if guardrails.IsAllowedPath(p) {
			allowed = append(allowed, p)
		}
	}
	return allowed
}

func (s *isolatedSandbox) SearchFiles(pattern string, path, fileType *string, caseSensitive *bool) (results []string, err error) {
	effectivePath := "."
	if path != nil && *path != "" {
		effectivePath = *path
	}

	if !guardrails.IsAllowedPath(effectivePath) {
		return nil, fmt.Errorf("access to path %q is not allowed", effectivePath)
	}

	req := searchFilesRequest{Path: effectivePath, Pattern: pattern, IgnoreCase: true}
	if fileType != nil {
		req.Type = *fileType
	}
	if caseSensitive != nil {
		req.IgnoreCase = !*caseSensitive
	}

	resp, err := s.agent.call(containerRequest{SearchFiles: &req})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("search failed: %s", resp.Error)
	}

	return resp.Results, nil
}

func (s *isolatedSandbox) WriteFile(path string, bs []byte) (err error) {
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	resp, err := s.agent.call(containerRequest{WriteFile: &writeFileRequest{Path: path, Content: base64Encode(bs)}})
	if err != nil {
		return fmt.Errorf("failed to write file %q: %w", path, err)
	}
	if resp.Error != "" {
		return fmt.Errorf("failed to write file %q: %s", path, resp.Error)
	}

	return nil
}

func (s *isolatedSandbox) PatchFile(path, diff string) (bs []byte, err error) {
	if !guardrails.IsAllowedPath(path) {
		return nil, fmt.Errorf("access to path %q is not allowed", path)
	}

	// Tolerate a missing file, same as the shell's `cat path || true` would: a diff is
	// allowed to create a brand new file.
	original, _ := s.ReadFile(path)

	patched, err := difftool.Apply(string(original), diff)
	if err != nil {
		return nil, fmt.Errorf("failed to apply diff to %q: %v", path, err)
	}

	if err = s.WriteFile(path, []byte(patched)); err != nil {
		return nil, err
	}

	return []byte(patched), nil
}

func (s *isolatedSandbox) RemoveFile(path string) (err error) {
	if !guardrails.IsAllowedPath(path) {
		return fmt.Errorf("access to path %q is not allowed", path)
	}

	resp, err := s.agent.call(containerRequest{RemoveFile: &removeFileRequest{Path: path}})
	if err != nil {
		return fmt.Errorf("failed to remove file %q: %w", path, err)
	}
	if resp.Error != "" {
		return fmt.Errorf("failed to remove file %q: %s", path, resp.Error)
	}

	return nil
}

func (s *isolatedSandbox) ShellCommand(command []string) (output string, exitCode int, err error) {
	resp, err := s.agent.call(containerRequest{Shell: &shellRequest{Command: command}})
	if err != nil {
		return "", 0, err
	}
	if resp.Error != "" {
		return "", 0, fmt.Errorf("%s", resp.Error)
	}

	bs, err := base64Decode(resp.Content)
	if err != nil {
		return "", 0, fmt.Errorf("failed to decode command output: %w", err)
	}

	return string(bs), resp.ExitCode, nil
}

// Close shuts down this sandbox's container, if one is running.
func (s *isolatedSandbox) Close() {
	s.agent.Close()
}

func validateDockerDependency() {
	if _, err := exec.LookPath("docker"); err != nil {
		panic("docker is not installed or not in PATH")
	}
}

const containerFSRoot = "containerfs"

var (
	//go:embed containerfs/**
	containerFS embed.FS
)

func buildContainer() (string, error) {
	tui.Printf("%% Buiding sandbox docker container:")

	hash, err := containerFSHash()
	if err != nil {
		return "", fmt.Errorf("failed to hash sandbox container filesystem: %v", err)
	}

	containerImage := fmt.Sprintf("ucode/sandbox:%s", hash[:8])
	if imageExists(containerImage) {
		tui.Printf("%% → %q (cached)\n", containerImage)
		return containerImage, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}

	buildContextDir := filepath.Join(wd, ".agents", "sandbox", "containerfs")
	if err = os.RemoveAll(buildContextDir); err != nil {
		return "", fmt.Errorf("failed to clean build context directory %q: %v", buildContextDir, err)
	}
	if err = os.MkdirAll(buildContextDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create build context directory %q: %v", buildContextDir, err)
	}

	if err = copyContainerFS(buildContextDir); err != nil {
		return "", fmt.Errorf("failed to prepare build context %q: %v", buildContextDir, err)
	}

	tui.Printf("%% docker buildx build --load -t %s %s", containerImage, buildContextDir)
	cmd := exec.Command("docker", "buildx", "build", "--load", "-t", containerImage, buildContextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build docker image: %v", err)
	}

	tui.Printf("%% → %q\n", containerImage)
	return containerImage, nil
}

// copyContainerFS copies the embedded container build context (containerFS, rooted at
// containerFSRoot) onto disk at destDir, since `docker buildx build` needs a real directory.
func copyContainerFS(destDir string) error {
	return fs.WalkDir(containerFS, containerFSRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(strings.TrimPrefix(path, containerFSRoot), "/")
		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := containerFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

// containerFSHash hashes the embedded container build context's paths and contents, so the
// resulting digest changes whenever the Dockerfile, the sandbox agent's source, or any other
// build input changes.
func containerFSHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(containerFS, containerFSRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := containerFS.ReadFile(path)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprint(h, path)
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// imageExists reports whether a docker image with the given tag already exists locally,
// so we can skip rebuilding the sandbox container when nothing has changed.
func imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	return cmd.Run() == nil
}
