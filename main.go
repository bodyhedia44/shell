package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main() {

	for {
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			os.Exit(1)
		}

		handleCommands(command)
	}

}

func handleCommands(input string) {
	command, err := parseCommand(input)

	if err != nil {
		fmt.Println(err.Error())
	}

	var outputFile *os.File
	for i, arg := range command {
		if (arg == ">" || arg == "1>") && i+1 < len(command) {
			outputFile, err = os.Create(command[i+1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error creating file:", err)
				continue
			}
			command = command[:i]
			break
		}
	}
	if outputFile != nil {
		defer outputFile.Close()
		originalStdout := os.Stdout
		os.Stdout = outputFile
		defer func() { os.Stdout = originalStdout }()
	}

	switch command[0] {
	case "exit":
		os.Exit(0)
	case "echo":
		fmt.Println(strings.Join(command[1:], " "))
	case "type":
		handleTypeCommand(command[1])
	case "pwd":
		currentWorkingDirectory, _ := os.Getwd()
		fmt.Println(currentWorkingDirectory)
	case "cd":
		handleCDCommand(command[1])
	default:
		file, _ := findBinInPath(command[0])
		if file != "" {
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		} else {
			fmt.Println(command[0] + ": command not found")
		}

		if outputFile != nil {
			os.Stdout = os.NewFile(uintptr(syscall.Stdout), "/dev/stdout")
		}
	}
}

func parseCommand(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)
	for _, r := range input {
		switch {
		case inQuote:
			if r == quoteChar {
				inQuote = false
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = true
			quoteChar = r
		case r == ' ' || r == '\n':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unclosed quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil

	// return strings.Split(n, " ")
}

func handleTypeCommand(command string) {
	type_helper := map[string]string{
		"echo": "echo is a shell builtin",
		"type": "type is a shell builtin",
		"exit": "exit is a shell builtin",
		"pwd":  "pwd is a shell builtin",
		"cd":   "cd is a shell builtin",
	}

	val, ok := type_helper[command]

	if ok {
		fmt.Println(val)
	} else {
		if file, exists := findBinInPath(command); exists {
			fmt.Fprintf(os.Stdout, "%s is %s\n", command, file)
			return
		}
		fmt.Println(command + ": not found")
	}

}

func handleCDCommand(dir string) {
	path := strings.Replace(dir, "~", os.Getenv("HOME"), 1)
	err := os.Chdir(path)
	if err != nil {
		fmt.Fprintf(os.Stdout, "cd: %s: No such file or directory\n", dir)
	}
}

func findBinInPath(bin string) (string, bool) {
	paths := os.Getenv("PATH")
	for _, path := range strings.Split(paths, ":") {
		file := path + "/" + bin
		if _, err := os.Stat(file); err == nil {
			return file, true
		}
	}
	return "", false
}

func pipeLineSuuport(line string) {
	if strings.Contains(line, "|") {
		parts := strings.SplitN(line, "|", 2)
		leftLine := strings.TrimSpace(parts[0])
		rightLine := strings.TrimSpace(parts[1])
		leftArgs := strings.Fields(leftLine)
		rightArgs := strings.Fields(rightLine)
		if len(leftArgs) == 0 || len(rightArgs) == 0 {
			fmt.Fprintln(os.Stderr, "bash: invalid pipeline")
		}
		cmd1 := exec.Command(leftArgs[0], leftArgs[1:]...)
		cmd2 := exec.Command(rightArgs[0], rightArgs[1:]...)
		// create pipe
		pipeReader, pipeWriter, err := os.Pipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pipe error: %v\n", err)
			os.Exit(1)
		}
		cmd1.Stdout = pipeWriter
		cmd1.Stderr = os.Stderr
		cmd2.Stdin = pipeReader
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		err1 := cmd1.Start()
		err2 := cmd2.Start()
		pipeWriter.Close() // allow cmd1 to signal EOF
		cmd1.Wait()
		pipeReader.Close() // allow cmd2 to finish
		cmd2.Wait()
		if err1 != nil || err2 != nil {
			fmt.Fprintln(os.Stderr, "bash: error in pipeline execution")
		}
	}
}
