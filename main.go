package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

var (
	progName string  // The name of this executable.
	subCmd   command // The active sub-command.

	dbPath    = "data.db" // The path to the DB file.
	cmdHandle string      // The handle for the cmd to modify or execute.
)

// command is the type of a valid sub-command.
type command string

// Valid sub-commands.
const (
	deleteCommand command = "delete"
	runCommand    command = "run"
	saveCommand   command = "save"
)

func init() {
	// Extract the program name from the path.
	path := strings.Split(os.Args[0], string(os.PathSeparator))
	progName = path[len(path)-1]

	// Define the usage message.
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global flags, ...] command [cmd flags, ...]\n\n", progName)
		fmt.Fprintln(os.Stderr, "Global flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nThe commands are:")
		fmt.Fprintln(os.Stderr, "  delete\tdelete a saved cmd")
		fmt.Fprintln(os.Stderr, "  run   \trun a saved cmd")
		fmt.Fprintln(os.Stderr, "  save  \tsave a new or update an existing cmd")
	}

	// Parse general arguments.
	flag.StringVar(&dbPath, "db", dbPath, "The database `path`")
	flag.Parse()

	// Parse sub-command name and arguments.
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing command name\n")
		flag.Usage()
		os.Exit(2)
	}
	switch command(args[0]) {
	case deleteCommand:
		subCmd = deleteCommand
		initCmdDelete(args[1:])
	case runCommand:
		subCmd = runCommand
		initCmdRun(args[1:])
	case saveCommand:
		subCmd = saveCommand
		initCmdSave(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
}

func initCmdDelete(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: delete [flags] <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[len(args)-1]
}

// initCmdRun parses arguments specific to command 'run'.
func initCmdRun(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: run [flags] <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[len(args)-1]
}

func initCmdSave(args []string) {
	flags := flag.NewFlagSet("save", flag.ExitOnError)

	flags.StringVar(&cmdHandle, "name", "", "The name used to refer to the saved cmd")

	if err := flags.Parse(args); err != nil || cmdHandle == "" {
		fmt.Fprintf(os.Stderr, "Usage: save [flags] cmd [<cmd flags>, ...]\n")
		flags.PrintDefaults()
		os.Exit(2)
	}
}

func main() {
	// Run the selected sub-command.
	var status int
	switch subCmd {
	case deleteCommand:
		// TODO
	case runCommand:
		status = execCmdRun()
	case saveCommand:
		// TODO
	}
	os.Exit(status)
}

// execCmdRun executes sub-command 'run'.
func execCmdRun() int {
	status := runCmd("sleep", "5s") // TODO pass cmdName and args
	return status
}

// runCmd calls runCmdAsync and waits for the child process to complete. Listens
// for interrupts SIGINT and SIGTERM and forwards them to the child.
//
// Attempts to return the process' exit status.
func runCmd(cmdName string, arg ...string) int {
	// Disable default behaviour and pass SIGINT and SIGTERM to child process.
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)

	// Start the requested process.
	exitCh, err := runCmdAsync(interruptCh, cmdName, arg...)
	if err != nil {
		log.Printf("%s failed to start: %v", cmdHandle, err)
		return 1
	}

	// Wait for the child process to exit.
	var exitStatus int
	err = <-exitCh
	if err != nil {
		log.Printf("%s exited with error: %v", cmdHandle, err)
		// Try to determine the exit status of the child process.
		switch err := err.(type) {
		case *exec.ExitError:
			if s, ok := err.Sys().(syscall.WaitStatus); ok {
				exitStatus = s.ExitStatus()
			} else {
				exitStatus = 1
			}
		default:
			exitStatus = 1 // Unknown reason.
		}
	}
	return exitStatus
}

// runCmdAsync starts a new process cmdName, passing arguments and connecting
// the current process's stdin, stdout and stderr to it. Returns immediately,
// not waiting for the child process to exit.
//
// signalCh can be used to send a signal to the child process.
//
// Returns an error if the process failed to be started. The error channel, on
// the other hand, is closed when the child process has exited, first passing
// any non-nil error returned by exec.Command.Wait.
func runCmdAsync(signalCh <-chan os.Signal, cmdName string, arg ...string) (<-chan error, error) {
	cmd := exec.Command(cmdName, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process.
	exitCh := make(chan error, 1)
	err := cmd.Start()
	if err != nil {
		close(exitCh)
		return exitCh, err
	}

	// Wait for it to exit.
	waitCh := make(chan struct{})
	go func() {
		err := cmd.Wait()
		if err != nil {
			exitCh <- err
		}
		// Close exitCh to signal that the process has exited to the outside.
		close(exitCh)
		// And close waitCh to notify local listeners without them consuming the
		// error value from exitCh.
		close(waitCh)
	}()

	// Forward all signals to the child process.
	go func() {
		var done bool
		for !done {
			select {
			case s := <-signalCh:
				err := cmd.Process.Signal(s)
				if err != nil {
					log.Printf("Failed to forward signal to %s: %v", cmdHandle, err)
				}
			case <-waitCh:
				done = true
			}
		}
	}()

	return exitCh, nil
}
