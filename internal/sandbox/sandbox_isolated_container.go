package sandbox

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/kapitanov/ucode/internal/tui"
)

const (
	// sandboxContainerLabel marks a container as belonging to a ucode sandbox, so orphaned
	// containers can be found independently of the image tag.
	sandboxContainerLabel = "ucode.sandbox"
	// sandboxContainerPIDLabel records the PID of the process that started the container, so
	// a future ucode process can tell whether the owner is still alive.
	sandboxContainerPIDLabel = "ucode.sandbox.pid"

	// maxCommandRetries bounds how many times a single call is retried (restarting the
	// container each time) before giving up and returning an error.
	maxCommandRetries = 3
)

// Request/response types mirror the protocol implemented by containerfs/main.go. The two sides
// can't share Go types directly (the container runs its own, independent module), so keep any
// change to one in sync with the other.

type containerRequest struct {
	ReadFile    *readFileRequest    `json:"read_file,omitempty"`
	ListFiles   *listFilesRequest   `json:"list_files,omitempty"`
	SearchFiles *searchFilesRequest `json:"search_files,omitempty"`
	WriteFile   *writeFileRequest   `json:"write_file,omitempty"`
	RemoveFile  *removeFileRequest  `json:"remove_file,omitempty"`
	Shell       *shellRequest       `json:"shell,omitempty"`
}

type readFileRequest struct {
	Path string `json:"path"`
}

type listFilesRequest struct {
	Dir string `json:"dir"`
}

type searchFilesRequest struct {
	Path       string `json:"path"`
	Pattern    string `json:"pattern"`
	Type       string `json:"type,omitempty"`
	IgnoreCase bool   `json:"ignore_case,omitempty"`
}

type writeFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type removeFileRequest struct {
	Path string `json:"path"`
}

type shellRequest struct {
	Command []string `json:"command"`
}

type containerResponse struct {
	Content  string   `json:"content,omitempty"`
	Dirs     []string `json:"dirs,omitempty"`
	Files    []string `json:"files,omitempty"`
	Results  []string `json:"results,omitempty"`
	ExitCode int      `json:"exit_code,omitempty"`
	Error    string   `json:"error,omitempty"`
}

func base64Encode(bs []byte) string { return base64.StdEncoding.EncodeToString(bs) }

func base64Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// containerAgent owns the single long-lived sandbox container for this process and speaks the
// newline-delimited JSON protocol to it over stdin/stdout, (re)starting it on demand.
type containerAgent struct {
	wd             string
	containerImage string

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

func newContainerAgent(wd, containerImage string) *containerAgent {
	killOrphanedContainers()

	return &containerAgent{
		wd:             wd,
		containerImage: containerImage,
	}
}

// call sends req to the sandbox container and returns its reply, starting the container if it
// isn't running yet. If the call fails (e.g. the container died), it is retried - restarting
// the container each time - up to maxCommandRetries times.
func (a *containerAgent) call(req containerRequest) (containerResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var lastErr error
	for attempt := 1; attempt <= maxCommandRetries; attempt++ {
		if err := a.ensureRunningLocked(); err != nil {
			lastErr = err
			continue
		}

		resp, err := a.callOnceLocked(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		tui.Printf("%% Sandbox container call failed (attempt %d/%d): %v", attempt, maxCommandRetries, err)
		a.stopLocked()
	}

	return containerResponse{}, fmt.Errorf("sandbox container call failed after %d retries: %w", maxCommandRetries, lastErr)
}

func (a *containerAgent) callOnceLocked(req containerRequest) (containerResponse, error) {
	line, err := json.Marshal(req)
	if err != nil {
		return containerResponse{}, fmt.Errorf("failed to encode request: %w", err)
	}

	line = append(line, '\n')
	if _, err = a.stdin.Write(line); err != nil {
		return containerResponse{}, fmt.Errorf("failed to write to sandbox container: %w", err)
	}

	if !a.stdout.Scan() {
		if err = a.stdout.Err(); err != nil {
			return containerResponse{}, fmt.Errorf("failed to read from sandbox container: %w", err)
		}
		return containerResponse{}, fmt.Errorf("sandbox container closed the connection")
	}

	var resp containerResponse
	if err = json.Unmarshal(a.stdout.Bytes(), &resp); err != nil {
		return containerResponse{}, fmt.Errorf("failed to parse sandbox container reply: %w", err)
	}

	return resp, nil
}

// ensureRunningLocked (re)starts the container if it is not currently running. A running
// container is only ever torn down by stopLocked (on a failed call, or on Close), so a non-nil
// cmd here always means the container is up.
func (a *containerAgent) ensureRunningLocked() error {
	if a.cmd != nil {
		return nil
	}

	cmd := exec.Command("docker", "run", "-i", "--rm",
		"--label", sandboxContainerLabel+"=1",
		"--label", fmt.Sprintf("%s=%d", sandboxContainerPIDLabel, os.Getpid()),
		"-v", fmt.Sprintf("%s:/mnt", a.wd),
		"-w", "/mnt",
		a.containerImage,
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open sandbox container stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open sandbox container stdout: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sandbox container: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = scanner
	return nil
}

// stopLocked tears down the current container connection, if any. Since the container is
// started with --rm, closing its stdin makes the agent process exit and docker removes the
// container on its own.
func (a *containerAgent) stopLocked() {
	if a.cmd == nil {
		return
	}

	_ = a.stdin.Close()
	_ = a.cmd.Wait()

	a.cmd = nil
	a.stdin = nil
	a.stdout = nil
}

// Close shuts down this agent's sandbox container, if one is running.
func (a *containerAgent) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}

// killOrphanedContainers removes sandbox containers left behind by a ucode process that is no
// longer running (e.g. after a crash), identified via sandboxContainerPIDLabel.
func killOrphanedContainers() {
	out, err := exec.Command("docker", "ps", "-aq", "--filter", "label="+sandboxContainerLabel+"=1").Output()
	if err != nil {
		return
	}

	for _, id := range strings.Fields(string(out)) {
		pidOut, err := exec.Command("docker", "inspect", "-f",
			fmt.Sprintf("{{ index .Config.Labels %q }}", sandboxContainerPIDLabel), id).Output()
		if err != nil {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(pidOut)))
		if err != nil || isProcessAlive(pid) {
			continue
		}

		tui.Printf("%% Removing orphaned sandbox container %s (owner pid %d is gone)", id, pid)
		_ = exec.Command("docker", "rm", "-f", id).Run()
	}
}

// isProcessAlive reports whether a process with the given PID is still running, by probing it
// with signal 0 (which performs the usual permission/existence checks without actually sending
// a signal).
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}
