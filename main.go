package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/alexflint/go-arg"
	. "github.com/logrusorgru/aurora"
	"io/ioutil"
	"log"
)

var args struct {
	TaskName[] string `arg:"positional"`
}

type Dofile struct {
	Description string
	Tasks map[string]Task
}

type Task struct {
	commands []string
}

func executeTask() {

}

func main() {
	arg.MustParse(&args)

	fileContents, err := ioutil.ReadFile("./Dofile")
	if err != nil {
		log.Fatal(err)
	}

	var doFile Dofile
	if _, err := toml.Decode(string(fileContents), &doFile); err != nil {
		log.Fatal(err)
	}

	for _, taskName := range args.TaskName {
		fmt.Println(Bold(Green("Executing task")), Bold(Cyan(taskName)))
	}
}
