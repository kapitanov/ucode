// Command sandbox-agent is the long-running process started inside the sandbox container. It
// reads newline-delimited JSON requests from stdin and writes newline-delimited JSON replies to
// stdout, implementing the file and shell operations the host-side sandbox needs.
//
// See ../sandbox_isolated_container.go on the host side for the matching request/response
// types and the docker invocation that starts this process. The two sides can't share Go types
// directly - this is its own, independent module - so keep any protocol change in sync on both
// ends.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type request struct {
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
	Type       string `json:"type"`
	IgnoreCase bool   `json:"ignore_case"`
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

type response struct {
	Content  string   `json:"content,omitempty"`
	Dirs     []string `json:"dirs,omitempty"`
	Files    []string `json:"files,omitempty"`
	Results  []string `json:"results,omitempty"`
	ExitCode int      `json:"exit_code,omitempty"`
	Error    string   `json:"error,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)

	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := handleLine(line)

		out, err := json.Marshal(resp)
		if err != nil {
			out, _ = json.Marshal(response{Error: err.Error()})
		}

		_, _ = writer.Write(out)
		_ = writer.WriteByte('\n')
		_ = writer.Flush()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "sandbox-agent: stdin read error:", err)
	}
}

func handleLine(line []byte) response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return response{Error: err.Error()}
	}

	switch {
	case req.ReadFile != nil:
		return handleReadFile(*req.ReadFile)
	case req.ListFiles != nil:
		return handleListFiles(*req.ListFiles)
	case req.SearchFiles != nil:
		return handleSearchFiles(*req.SearchFiles)
	case req.WriteFile != nil:
		return handleWriteFile(*req.WriteFile)
	case req.RemoveFile != nil:
		return handleRemoveFile(*req.RemoveFile)
	case req.Shell != nil:
		return handleShell(*req.Shell)
	default:
		return response{Error: "empty or unrecognized request"}
	}
}

func handleReadFile(r readFileRequest) response {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return response{Error: err.Error()}
	}

	return response{Content: base64.StdEncoding.EncodeToString(data)}
}

func handleListFiles(r listFilesRequest) response {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return response{Error: err.Error()}
	}

	var dirs, files []string
	for _, e := range entries {
		abs, err := filepath.Abs(filepath.Join(r.Dir, e.Name()))
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}

		if e.IsDir() {
			dirs = append(dirs, abs)
		} else {
			files = append(files, abs)
		}
	}

	return response{Dirs: dirs, Files: files}
}

func handleSearchFiles(r searchFilesRequest) response {
	args := []string{"--line-number", "--with-filename", "--color=never"}
	if r.IgnoreCase {
		args = append(args, "--ignore-case")
	}
	if r.Type != "" {
		args = append(args, "--type", r.Type)
	}

	path := r.Path
	if path == "" {
		path = "."
	}
	args = append(args, r.Pattern, path)

	output, err := exec.Command("rg", args...).CombinedOutput()
	if err != nil {
		// ripgrep returns exit code 1 when no matches are found, which is not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return response{}
		}
		return response{Error: fmt.Sprintf("%v: %s", err, output)}
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return response{}
	}

	return response{Results: strings.Split(outputStr, "\n")}
}

func handleWriteFile(r writeFileRequest) response {
	data, err := base64.StdEncoding.DecodeString(r.Content)
	if err != nil {
		return response{Error: err.Error()}
	}

	if err = os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return response{Error: err.Error()}
	}

	if err = os.WriteFile(r.Path, data, 0644); err != nil {
		return response{Error: err.Error()}
	}

	return response{}
}

func handleRemoveFile(r removeFileRequest) response {
	if err := os.RemoveAll(r.Path); err != nil {
		return response{Error: err.Error()}
	}

	return response{}
}

func handleShell(r shellRequest) response {
	if len(r.Command) == 0 {
		return response{Error: "empty command"}
	}

	args := append([]string{"-c"}, r.Command...)
	output, err := exec.Command("sh", args...).CombinedOutput()

	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return response{Error: err.Error()}
		}
		exitCode = exitErr.ExitCode()
	}

	return response{
		Content:  base64.StdEncoding.EncodeToString(output),
		ExitCode: exitCode,
	}
}
