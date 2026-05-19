package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"oh-my-gunnsama/remote/internal/observer"
	"oh-my-gunnsama/remote/internal/report"
	"oh-my-gunnsama/remote/internal/reportserver"
)

type observeInfo struct {
	Project string `json:"project"`
	Out     string `json:"out"`
	Served  bool   `json:"served"`
	URL     string `json:"url,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "missing command: use observe")
		return 1
	}
	switch args[0] {
	case "observe":
		return runObserve(ctx, args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "remote dev")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 1
	}
}

func runObserve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("observe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	project := fs.String("project", "", "project root to observe")
	out := fs.String("out", "", "output report directory outside project")
	once := fs.Bool("once", false, "generate once and exit")
	serve := fs.Bool("serve", false, "serve report and refresh periodically")
	host := fs.String("host", "127.0.0.1", "serve host")
	port := fs.Int("port", 8787, "serve port")
	refresh := fs.Duration("refresh", 5*time.Second, "serve refresh interval")
	printJSON := fs.Bool("json", false, "print machine-readable result")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *project == "" {
		fmt.Fprintln(stderr, "--project is required")
		return 1
	}
	if *once && *serve {
		fmt.Fprintln(stderr, "--once and --serve are mutually exclusive")
		return 1
	}
	if *serve && *refresh < time.Second {
		fmt.Fprintln(stderr, "--refresh must be at least 1s")
		return 1
	}
	absProject, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if info, err := os.Stat(absProject); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "project does not exist or is not a directory: %s\n", absProject)
		return 1
	}
	resolvedOut, err := observer.ResolveOutput(absProject, *out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	scanAndWrite := func() error {
		state, err := observer.NewScanner(observer.ScannerOptions{}).Scan(absProject)
		if err != nil {
			return err
		}
		return report.Write(resolvedOut, state)
	}
	if err := scanAndWrite(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !*serve {
		writeInfo(stdout, observeInfo{Project: absProject, Out: resolvedOut, Served: false}, *printJSON)
		return 0
	}
	addr := fmt.Sprintf("%s:%d", *host, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(stderr, "serve failed: %v\n", err)
		return 1
	}
	actual := listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://%s:%d", *host, actual.Port)
	writeInfo(stdout, observeInfo{Project: absProject, Out: resolvedOut, Served: true, URL: url}, *printJSON)
	reportSrv := reportserver.NewWithRefresh(resolvedOut, scanAndWrite)
	go reportSrv.RefreshLoop(ctx, *refresh)
	server := &http.Server{Handler: reportSrv.Handler()}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "serve failed: %v\n", err)
		return 1
	}
	return 0
}

func writeInfo(stdout io.Writer, info observeInfo, printJSON bool) {
	if printJSON {
		_ = json.NewEncoder(stdout).Encode(info)
		return
	}
	fmt.Fprintf(stdout, "Remote Observer\nProject: %s\nOut: %s\n", info.Project, info.Out)
	if info.Served {
		fmt.Fprintf(stdout, "URL: %s\n", info.URL)
	}
}
