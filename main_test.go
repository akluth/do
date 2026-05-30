package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestTaskNamesInFileOrder(t *testing.T) {
	var doFile Dofile
	metadata, err := toml.Decode(`
description = "test"

[tasks]
  [tasks.build]
  commands = ["build"]

  [tasks.clean]
  commands = ["clean"]
`, &doFile)
	if err != nil {
		t.Fatal(err)
	}

	got := taskNamesInFileOrder(metadata, doFile.Tasks)
	want := []string{"build", "clean"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("task order = %v, want %v", got, want)
	}
}

func TestExecuteTaskRunsSubtasksBeforeCommands(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	var out bytes.Buffer
	runner := testExecutor(&out, Dofile{
		Tasks: map[string]task{
			"build": {
				Tasks:    []string{"clean"},
				Commands: []string{helperCommand("print", "build")},
				Output:   true,
				Piped:    true,
			},
			"clean": {
				Commands: []string{helperCommand("print", "clean")},
				Output:   true,
				Piped:    true,
			},
		},
	})

	if err := runner.executeTask("build"); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "clean\nbuild\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSilentSuppressesCommandOutput(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	var out bytes.Buffer
	runner := testExecutor(&out, Dofile{
		Tasks: map[string]task{
			"quiet": {
				Commands: []string{helperCommand("print", "hidden")},
				Output:   true,
				Piped:    true,
			},
		},
	})
	runner.silent = true

	if err := runner.executeTask("quiet"); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); got != "" {
		t.Fatalf("silent output = %q, want empty", got)
	}
}

func TestCommandFailureReturnsErrorAndKeepsBufferedOutput(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	var out bytes.Buffer
	runner := testExecutor(&out, Dofile{
		Tasks: map[string]task{
			"fail": {
				Commands: []string{helperCommand("fail")},
				Output:   true,
			},
		},
	})

	err := runner.executeTask("fail")
	if err == nil {
		t.Fatal("expected command failure")
	}
	if got := out.String(); !strings.Contains(got, "before fail") {
		t.Fatalf("output = %q, want buffered command output", got)
	}
}

func TestTaskCycleReturnsError(t *testing.T) {
	var out bytes.Buffer
	runner := testExecutor(&out, Dofile{
		Tasks: map[string]task{
			"loop": {Tasks: []string{"loop"}},
		},
	})

	err := runner.executeTask("loop")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v, want cycle error", err)
	}
}

func testExecutor(out *bytes.Buffer, doFile Dofile) *executor {
	return &executor{
		doFile:     doFile,
		dirPrefix:  ".",
		out:        out,
		processing: make(map[string]bool),
	}
}

func helperCommand(args ...string) string {
	parts := []string{strconv.Quote(os.Args[0]), "-test.run=TestHelperProcess", "--"}
	for _, arg := range args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	args = args[1:]

	switch args[0] {
	case "print":
		fmt.Println(args[1])
	case "fail":
		fmt.Println("before fail")
		os.Exit(7)
	default:
		os.Exit(2)
	}

	os.Exit(0)
}
