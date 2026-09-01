package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		time.Sleep(24 * time.Hour)
		return
	}
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "expected serve command")
		os.Exit(2)
	}
	mustWriteFile(os.Getenv("FAKE_OPENCODE_PID_FILE"), strconv.Itoa(os.Getpid()))
	startChild()
	if delay := os.Getenv("FAKE_OPENCODE_DELAY"); delay != "" {
		d, err := time.ParseDuration(delay)
		if err != nil {
			panic(err)
		}
		time.Sleep(d)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(agents)}
	go func() { _ = server.Serve(listener) }()
	fmt.Fprintf(os.Stdout, "opencode server listening on http://%s\n", listener.Addr())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
}

func agents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/agent" || r.URL.Query().Get("directory") != os.Getenv("FAKE_OPENCODE_EXPECT_DIRECTORY") {
		http.Error(w, "unexpected request", http.StatusBadRequest)
		return
	}
	mustWriteFile(os.Getenv("FAKE_OPENCODE_QUERY_FILE"), r.URL.RawQuery)
	data, err := os.ReadFile(os.Getenv("FAKE_OPENCODE_AGENTS_FILE"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func startChild() {
	path := os.Getenv("FAKE_OPENCODE_CHILD_PID_FILE")
	if path == "" {
		return
	}
	cmd := exec.Command(os.Args[0], "child")
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	mustWriteFile(path, strconv.Itoa(cmd.Process.Pid))
}

func mustWriteFile(path, value string) {
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		panic(err)
	}
}
