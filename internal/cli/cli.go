package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/minkin/web-config-guard/internal/runner"
	"github.com/minkin/web-config-guard/internal/server"
)

const (
	exitOK       = 0
	exitIssues   = 1
	exitBadUsage = 2
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		silent   bool
		useStdin bool
		serve    bool
		addr     string
		output   string
	)

	flags := flag.NewFlagSet("web-config-guard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&silent, "s", false, "do not exit with error when problems are found")
	flags.BoolVar(&silent, "silent", false, "do not exit with error when problems are found")
	flags.BoolVar(&useStdin, "stdin", false, "read configuration from standard input")
	flags.BoolVar(&serve, "serve", false, "run HTTP REST API server")
	flags.StringVar(&addr, "addr", ":8080", "HTTP listen address")
	flags.StringVar(&output, "format", "text", "output format: text or json")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  web-config-guard [flags] <config-file-or-directory>")
		fmt.Fprintln(stderr, "  web-config-guard --stdin [flags]")
		fmt.Fprintln(stderr, "  web-config-guard --serve [--addr :8080]")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return exitBadUsage
	}

	run := runner.New()
	if serve {
		if flags.NArg() != 0 || useStdin {
			fmt.Fprintln(stderr, "--serve cannot be combined with a file path or --stdin")
			return exitBadUsage
		}
		return serveHTTP(addr, run, stderr)
	}

	var (
		result runner.Result
		err    error
	)
	switch {
	case useStdin:
		if flags.NArg() != 0 {
			fmt.Fprintln(stderr, "--stdin cannot be combined with a positional path")
			return exitBadUsage
		}
		result, err = run.CheckReader(stdin, "stdin.yaml")
	case flags.NArg() == 1:
		result, err = run.CheckPath(ctx, flags.Arg(0))
	default:
		flags.Usage()
		return exitBadUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	if err := writeOutput(stdout, output, result); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	if len(result.Problems) > 0 && !silent {
		return exitIssues
	}
	return exitOK
}

func serveHTTP(addr string, run runner.Runner, stderr io.Writer) int {
	handler := server.New(run).Handler()
	fmt.Fprintf(stderr, "Listening on %s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}
	return exitOK
}

func writeOutput(writer io.Writer, format string, result runner.Result) error {
	switch strings.ToLower(format) {
	case "text":
		_, err := io.WriteString(writer, runner.FormatText(result.Problems))
		return err
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
