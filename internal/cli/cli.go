package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/minkin/web-config-guard/internal/config"
	"github.com/minkin/web-config-guard/internal/guard"
	"github.com/minkin/web-config-guard/internal/runner"
	"github.com/minkin/web-config-guard/internal/server"
)

const (
	exitOK       = 0
	exitIssues   = 1
	exitBadUsage = 2
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--healthcheck" {
		c := &http.Client{Timeout: time.Second}
		resp, err := c.Get("http://127.0.0.1:8080/health/live")
		if err != nil {
			return 1
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return 1
		}
		return 0
	}
	var (
		silent     bool
		useStdin   bool
		serve      bool
		addr       string
		output     string
		configPath string
		failOn     string
		logFormat  string
		logLevel   string
	)
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			configPath = args[i+1]
		}
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
		}
	}
	settings, err := config.LoadSettings(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	flags := flag.NewFlagSet("web-config-guard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&silent, "s", false, "do not exit with error when problems are found")
	flags.BoolVar(&silent, "silent", false, "do not exit with error when problems are found")
	flags.BoolVar(&useStdin, "stdin", false, "read configuration from standard input")
	flags.BoolVar(&serve, "serve", false, "run HTTP REST API server")
	flags.StringVar(&addr, "addr", settings.Server.Listen, "HTTP listen address")
	flags.StringVar(&output, "format", "text", "output format: text or json")
	flags.StringVar(&configPath, "config", configPath, "tool YAML settings")
	flags.StringVar(&failOn, "fail-on", settings.FailOn, "minimum severity that yields exit code 1: low, medium, high")
	flags.StringVar(&logFormat, "log-format", settings.Log.Format, "server log format: json or text")
	flags.StringVar(&logLevel, "log-level", settings.Log.Level, "server log level")
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
	settings.Server.Listen = addr
	settings.FailOn = failOn
	settings.Log.Format = logFormat
	settings.Log.Level = logLevel
	if err := settings.Validate(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	run := runner.New()
	if serve {
		if flags.NArg() != 0 || useStdin {
			fmt.Fprintln(stderr, "--serve cannot be combined with a file path or --stdin")
			return exitBadUsage
		}
		return serveHTTP(ctx, addr, run, settings, stderr)
	}

	var (
		result runner.Result
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

	if meetsThreshold(result.Problems, failOn) && !silent {
		return exitIssues
	}
	return exitOK
}

func meetsThreshold(problems []guard.Problem, threshold string) bool {
	rank := map[guard.Severity]int{guard.SeverityLow: 1, guard.SeverityMedium: 2, guard.SeverityHigh: 3}
	for _, p := range problems {
		if rank[p.Severity] >= rank[guard.Severity(strings.ToUpper(threshold))] {
			return true
		}
	}
	return false
}

func serveHTTP(ctx context.Context, addr string, run runner.Runner, settings config.Settings, stderr io.Writer) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	level := slog.LevelInfo
	switch settings.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var h slog.Handler = slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level})
	if settings.Log.Format == "text" {
		h = slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(h)
	handler := server.New(run, logger).Handler()
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	logger.Info("listening", "address", addr)
	go func() {
		<-ctx.Done()
		end, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(end)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}
	return exitOK
}

func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
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
