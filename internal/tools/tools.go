package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
)

func CheckDependencies() error {
	if _, err := exec.LookPath("rg"); err != nil {
		return fmt.Errorf("ripgrep (rg) is not installed or not in PATH")
	}
	return nil
}

func Definitions() []openrouter.Tool {
	var tools []openrouter.Tool
	for _, toolDef := range toolDefinitions {
		tools = append(tools, openrouter.Tool{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        toolDef.Name,
				Description: toolDef.Description,
				Strict:      true,
				Parameters:  toolDef.Parameters,
			},
		})
	}

	slices.SortFunc(tools, func(a, b openrouter.Tool) int {
		return strings.Compare(a.Function.Name, b.Function.Name)
	})

	return tools
}

func Execute(toolName string, toolArgs string) (string, error) {
	toolDef, ok := toolDefinitions[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q not found", toolName)
	}

	return toolDef.Execute(toolArgs)
}

func Describe(toolName string, toolArgs string) string {
	toolDef, ok := toolDefinitions[toolName]
	if !ok {
		return fmt.Sprintf("ERROR: tool %q not found", toolName)
	}

	return toolDef.Describe(toolArgs)
}

type toolDefinition struct {
	Name        string
	Description string
	Parameters  any
	Execute     func(args string) (string, error)
	Describe    func(args string) string
}

var (
	toolDefinitions = make(map[string]toolDefinition)
)

func defineTool[T, V any](name, description string, execute func(args T) (V, error), describe func(args T) string) {
	schema, err := jsonschema.GenerateSchema[T]()
	if err != nil {
		panic(err)
	}

	toolDefinitions[name] = toolDefinition{
		Name:        name,
		Description: description,
		Parameters:  schema,
		Execute: func(rawArgs string) (string, error) {
			var args T
			if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
				return "", fmt.Errorf("invalid arguments for tool %q: %v", name, err)
			}

			v, err := execute(args)
			if err != nil {
				return "", err
			}

			result, err := json.Marshal(v)
			if err != nil {
				return "", err
			}

			return string(result), nil
		},
		Describe: func(rawArgs string) string {
			var args T
			if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
				return fmt.Sprintf("ERROR: invalid arguments for tool %q: %v", name, err)
			}

			return describe(args)
		},
	}
}

func normalizePath(path string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	normalizedPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	// Check if normalizedPath is within wd (allow exact match or subdirectories)
	if normalizedPath != wd {
		if !strings.HasPrefix(normalizedPath, wd+string(filepath.Separator)) {
			return "", fmt.Errorf("directory %q is outside of the working directory", path)
		}
	}

	return normalizedPath, nil
}
