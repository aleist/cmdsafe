// This file implements sub-command 'run'.

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"bitbucket.org/aleist/cmdsafe/protobuf/data"
)

// doCmdRun performs sub-command 'run' and returns an exit status.
func doCmdRun() int {
	// Load command from DB.
	// TODO

	// Parse command info.
	cmdInfo = &data.Command{}
	// TODO
	cmdInfo.Name = "ls"
	cmdInfo.Args = []string{"-l"}

	// Run the command.
	status := runCmd(cmdInfo.GetName(), cmdInfo.GetArgs()...)
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
