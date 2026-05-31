package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wsfs-core-mount-blackbox/cases"
	"wsfs-core-mount-blackbox/harness"
)

func main() {
	var (
		wsfsBin       = flag.String("wsfs-bin", defaultWsfsBin(), "Path to wsfs executable")
		workRoot      = flag.String("work-root", defaultWorkRoot(), "Directory for per-case work artifacts")
		keepWork      = flag.Bool("keep-work", false, "Keep per-case work directory on success too")
		listCases     = flag.Bool("list", false, "List available cases and exit")
		structTimeout = flag.Int("struct-timeout", 0, "Mount --struct-timeout seconds")
		timeout       = flag.Duration("timeout", 30*time.Second, "Per-case timeout")
	)
	flag.Parse()

	if *listCases {
		for _, name := range cases.Names() {
			fmt.Println(name)
		}
		return
	}

	selected, err := cases.Lookup(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	runner, err := harness.NewRunner(harness.Config{
		WsfsBin:       *wsfsBin,
		WorkRoot:      *workRoot,
		StructTimeout: *structTimeout,
		KeepWork:      *keepWork,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	failures := 0
	skips := 0
	for _, c := range selected {
		fmt.Printf("=== RUN   %s\n", c.Name())
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		result := runner.RunCase(ctx, c)
		cancel()

		if result.Err == nil {
			fmt.Printf("--- PASS: %s\n", c.Name())
			continue
		}
		if harness.IsSkip(result.Err) {
			skips++
			fmt.Printf("--- SKIP: %s\n", c.Name())
			fmt.Printf("    reason: %v\n", result.Err)
			continue
		}

		failures++
		fmt.Printf("--- FAIL: %s\n", c.Name())
		fmt.Printf("    workdir: %s\n", result.WorkDir)
		fmt.Printf("    error: %v\n", result.Err)
		printLogTail("server.log", filepath.Join(result.WorkDir, "logs", "server.log"))
		printLogTail("mount.log", filepath.Join(result.WorkDir, "logs", "mount.log"))
	}

	if failures != 0 {
		fmt.Printf("\nFAIL: %d case(s) failed", failures)
		if skips != 0 {
			fmt.Printf(", %d skipped", skips)
		}
		fmt.Println()
		os.Exit(1)
	}
	fmt.Printf("\nPASS: %d case(s)", len(selected)-skips)
	if skips != 0 {
		fmt.Printf(", %d skipped", skips)
	}
	fmt.Println()
}

func printLogTail(label string, path string) {
	fmt.Printf("    %s tail:\n", label)
	if err := harness.CopyTail(os.Stdout, path, 4096); err != nil {
		fmt.Printf("      <unable to read %s: %v>\n", path, err)
		return
	}
	if !strings.HasSuffix(path, "\n") {
		fmt.Println()
	}
}

func defaultWorkRoot() string {
	return filepath.Clean(filepath.Join("..", "..", "run", "mount-blackbox"))
}

func defaultWsfsBin() string {
	if v := os.Getenv("WSFS_BIN"); v != "" {
		return v
	}
	for _, candidate := range []string{
		filepath.Clean(filepath.Join("..", "..", "build", "wsfs-linux-amd64")),
		filepath.Clean(filepath.Join("..", "..", "build", "wsfs")),
		"wsfs",
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "wsfs"
}
