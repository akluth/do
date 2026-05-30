// Copyright (c) 2019-2024 Alexander Kluth
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alexflint/go-arg"
	"github.com/anmitsu/go-shlex"
	"github.com/logrusorgru/aurora/v4"
)

var args struct {
	TaskName   []string `arg:"positional"`
	File       string   `arg:"-f" help:"Path to Dofile when it's not in current directory"`
	Init       bool     `arg:"-i" help:"Create a skeleton Dofile"`
	NotVerbose bool     `arg:"-n" help:"Not verbose mode, print only command output"`
	Silent     bool     `arg:"-s" help:"Silent mode, print nothing except errors"`
}

type Dofile struct {
	Description string
	Tasks       map[string]task
}

type task struct {
	Commands []string
	Tasks    []string
	Output   bool
	Piped    bool
}

type executor struct {
	doFile     Dofile
	dirPrefix  string
	out        io.Writer
	verbose    bool
	silent     bool
	processing map[string]bool
}

func parseCommand(command string) ([]string, error) {
	parts, err := shlex.Split(strings.TrimSpace(command), true)
	if err != nil {
		return nil, err
	}

	return parts, nil
}

func (e *executor) executeTask(taskName string) error {
	currentTask, found := e.doFile.Tasks[taskName]
	if !found {
		return fmt.Errorf("could not find task %q", taskName)
	}

	if e.processing[taskName] {
		return fmt.Errorf("task cycle detected at %q", taskName)
	}
	e.processing[taskName] = true
	defer delete(e.processing, taskName)

	if e.verbose {
		fmt.Fprintln(e.out, aurora.Bold("-> Executing task\t"), aurora.Bold(aurora.Cyan(taskName)))
	}

	for _, subtask := range currentTask.Tasks {
		if e.verbose {
			fmt.Fprintln(e.out, aurora.Bold(aurora.Cyan("-> Executing subtask\t")), aurora.Bold(subtask))
		}

		if err := e.executeTask(subtask); err != nil {
			return err
		}
	}

	for _, command := range currentTask.Commands {
		if e.verbose {
			fmt.Fprintln(e.out, "  ", aurora.Bold(aurora.Yellow(taskName)), "(", command, ")")
		}

		if err := e.executeCommand(currentTask, command); err != nil {
			return fmt.Errorf("task %q command %q failed: %w", taskName, command, err)
		}
	}

	return nil
}

func (e *executor) executeCommand(currentTask task, command string) error {
	tokens, err := parseCommand(command)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(tokens[0], tokens[1:]...)
	cmd.Dir = e.dirPrefix

	if !currentTask.Output || e.silent {
		return cmd.Run()
	}

	if currentTask.Piped {
		cmd.Stdout = e.out
		cmd.Stderr = e.out
		return cmd.Run()
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		_, _ = fmt.Fprintf(e.out, "\t%s", string(output))
	}
	return err
}

func taskNamesInFileOrder(metadata toml.MetaData, tasks map[string]task) []string {
	seen := make(map[string]bool, len(tasks))
	names := make([]string, 0, len(tasks))

	for _, key := range metadata.Keys() {
		if len(key) >= 2 && key[0] == "tasks" {
			name := key[1]
			if _, ok := tasks[name]; ok && !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
	}

	unordered := make([]string, 0)
	for name := range tasks {
		if !seen[name] {
			unordered = append(unordered, name)
		}
	}
	sort.Strings(unordered)
	names = append(names, unordered...)
	return names
}

func createDoFileSkeleton() {
	_, err := os.Stat("Dofile")
	if !os.IsNotExist(err) {
		fmt.Println(aurora.Red("Error: 'Dofile' already exists in current directory, aborting."))
		os.Exit(-1)
	}

	var Dofile = `
# A somewhat descriptive name for your project/Dofile
description = 'Dofile example'

# All tasks are listed here
[tasks]

	# Each tasks is defined by tasks.$TASKNAME
	[tasks.yourTaskName]
	
	# Here are all commands listed which shall be executed
	commands = [
		"$YOUR_COMMAND",
		"$ANOTHER_COMMAND --with $args"
	]

	# Setting output to true will print any stdout/stderr output of the executed programs to stdout
	output = true

	# Setting piped to true will print all output immediately via pipes to stdout/stderr, setting to false
	# will print the output of the commands _after_ their execution
	piped = false

	[tasks.subTasks]
	
	# You can combine tasks under one task name
	tasks = [
		"yourTaskName",
		"thisTaskDoesNotExistNow"
	]
`

	file, err := os.Create("Dofile")
	if err != nil {
		panic(err)
	}

	_, err = file.WriteString(Dofile)
	if err != nil {
		panic(err)
	}

	_ = file.Sync()

	fmt.Println(aurora.Green("Wrote Dofile to current directory. Edit it and then simply run 'do'."))
}

func main() {
	arg.MustParse(&args)

	var fileName = "./Dofile"
	var dirPrefix = "./"

	if args.Init {
		createDoFileSkeleton()
		os.Exit(0)
	}

	if args.File != "" {
		fileName = args.File
		dirPrefix = filepath.Dir(fileName)
	}

	fileContents, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	var doFile Dofile
	metadata, err := toml.Decode(string(fileContents), &doFile)
	if err != nil {
		log.Fatal(err)
	}

	if !args.Silent && !args.NotVerbose {
		fmt.Println(aurora.Bold(aurora.Green("\nExecuting tasks for")), doFile.Description)
		fmt.Println()
	}

	runner := executor{
		doFile:     doFile,
		dirPrefix:  dirPrefix,
		out:        os.Stdout,
		verbose:    !args.Silent && !args.NotVerbose,
		silent:     args.Silent,
		processing: make(map[string]bool),
	}

	if len(args.TaskName) > 0 {
		for _, taskName := range args.TaskName {
			if err := runner.executeTask(taskName); err != nil {
				fmt.Fprintln(os.Stderr, aurora.Bold(aurora.Red(err.Error())))
				os.Exit(1)
			}
		}
	} else {
		for _, taskName := range taskNamesInFileOrder(metadata, doFile.Tasks) {
			if err := runner.executeTask(taskName); err != nil {
				fmt.Fprintln(os.Stderr, aurora.Bold(aurora.Red(err.Error())))
				os.Exit(1)
			}
		}
	}

	if !args.Silent && !args.NotVerbose {
		fmt.Println(aurora.Bold(aurora.Green("\nExecuted all tasks for")), doFile.Description)
		fmt.Println()
	}
}
