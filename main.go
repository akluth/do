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
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alexflint/go-arg"
	"github.com/anmitsu/go-shlex"
	"github.com/logrusorgru/aurora/v4"
)

var args struct {
	TaskName   []string `arg:"positional"`
	File       string   `arg:"-f" help:"Path to Dofile whne it's not in current directory"`
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

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func parseCommand(command string) []string {
	parts, err := shlex.Split(strings.TrimSpace(command), true)
	if err != nil {
		log.Fatal(err)
	}

	return parts
}

func executeTask(doFile Dofile, dirPrefix string, taskName string) {
	if _, found := doFile.Tasks[taskName]; found {
		if !args.Silent && !args.NotVerbose {
			fmt.Println(aurora.Bold("-> Executing task\t"), aurora.Bold(aurora.Cyan(taskName)))
		}

		for _, command := range doFile.Tasks[taskName].Commands {
			if !args.Silent && !args.NotVerbose {
				fmt.Println("  ", aurora.Bold(aurora.Yellow(taskName)), "(", command, ")")
			}

			tokens := parseCommand(command)
			cmdName := tokens[0]
			tokens = remove(tokens, 0)

			cmd := exec.Command(cmdName, tokens...)
			cmd.Dir = dirPrefix

			if doFile.Tasks[taskName].Output {
				if doFile.Tasks[taskName].Piped {
					cmdReader, err := cmd.StdoutPipe()
					if err != nil {
						_, _ = fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
						os.Exit(1)
					}

					scanner := bufio.NewScanner(cmdReader)
					go func() {
						for scanner.Scan() {
							if args.Silent {
								fmt.Printf("%s\n", scanner.Text())
							} else {
								fmt.Printf("\t%s\n", scanner.Text())
							}
						}
					}()

					err = cmd.Start()
					if err != nil {
						_, _ = fmt.Fprintln(os.Stderr, "Error starting Cmd,", err)
						os.Exit(1)
					}

					err = cmd.Wait()
					if err != nil {
						_, _ = fmt.Fprintln(os.Stderr, "Error waiting for Cmd,", err)
						os.Exit(1)
					}
				} else {
					out, _ := cmd.CombinedOutput()
					fmt.Printf("\t%s", string(out))
				}
			} else {
				if err := cmd.Run(); err != nil {
					_, _ = fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			}
		}

		for _, task := range doFile.Tasks[taskName].Tasks {
			if !args.Silent && !args.NotVerbose {
				fmt.Println(aurora.Bold(aurora.Cyan("-> Executing subtask\t")), aurora.Bold(task))
			}

			executeTask(doFile, dirPrefix, task)
		}
	} else {
		fmt.Println(aurora.Bold(aurora.Red("Could not find task")), aurora.Bold(aurora.Yellow(taskName)), aurora.Bold(aurora.Red("aborting!")))
		os.Exit(-1)
	}
}

func createDoFileSkeleton() {
	_, err := os.Stat("Dofile")
	if !os.IsNotExist(err) {
		fmt.Println(aurora.Red("Error: 'Dofile' already exists in current directory, aborting."))
		os.Exit(-1)
	}

	var Dofile = `
# A somewhat descriptive name for your project/Dofile
desc = 'Dofile example'

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
		"yourTaskName".
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

	fileContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	var doFile Dofile
	if _, err := toml.Decode(string(fileContents), &doFile); err != nil {
		log.Fatal(err)
	}

	if !args.Silent && !args.NotVerbose {
		fmt.Println(aurora.Bold(aurora.Green("\nExecuting tasks for")), doFile.Description)
		fmt.Println()
	}

	if len(args.TaskName) > 0 {
		for _, taskName := range args.TaskName {
			executeTask(doFile, dirPrefix, taskName)
		}
	} else {
		for taskName := range doFile.Tasks {
			executeTask(doFile, dirPrefix, taskName)
		}
	}

	if !args.Silent && !args.NotVerbose {
		fmt.Println(aurora.Bold(aurora.Green("\nExecuted all tasks for")), doFile.Description)
		fmt.Println()
	}
}
