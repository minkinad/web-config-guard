package runner

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/minkin/web-config-guard/internal/config"
	"github.com/minkin/web-config-guard/internal/guard"
)

type Result struct {
	Problems []guard.Problem `json:"problems"`
}

type Runner struct {
	Checker guard.Checker
}

func New() Runner {
	return Runner{Checker: guard.NewChecker(nil)}
}

func (runner Runner) CheckBytes(data []byte, sourceName string) (Result, error) {
	value, err := config.Parse(data, sourceName)
	if err != nil {
		return Result{}, err
	}
	return Result{Problems: runner.Checker.Check(value)}, nil
}

func (runner Runner) CheckReader(reader io.Reader, sourceName string) (Result, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Result{}, err
	}
	return runner.CheckBytes(data, sourceName)
}

func (runner Runner) CheckPath(ctx context.Context, path string) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, err
	}
	if info.IsDir() {
		return runner.CheckDirectory(ctx, path)
	}
	return runner.checkFile(path, info)
}

func (runner Runner) CheckDirectory(ctx context.Context, root string) (Result, error) {
	var all []guard.Problem
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			if isHidden(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConfigFile(path) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		result, err := runner.checkFile(path, info)
		if err != nil {
			return err
		}
		all = append(all, result.Problems...)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	guard.SortProblems(all)
	return Result{Problems: all}, nil
}

func (runner Runner) checkFile(path string, info fs.FileInfo) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}

	result, err := runner.CheckBytes(data, path)
	if err != nil {
		return Result{}, err
	}

	for i := range result.Problems {
		result.Problems[i].File = path
	}
	result.Problems = append(result.Problems, guard.CheckFilePermissions(path, info, guard.HasSecretProblems(result.Problems))...)
	guard.SortProblems(result.Problems)
	return result, nil
}

func isConfigFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

func FormatText(problems []guard.Problem) string {
	if len(problems) == 0 {
		return "No problems found.\n"
	}

	var builder strings.Builder
	for _, problem := range problems {
		builder.WriteString(problem.Text())
		builder.WriteByte('\n')
	}
	builder.WriteString(fmt.Sprintf("Found %d problem(s).\n", len(problems)))
	return builder.String()
}
