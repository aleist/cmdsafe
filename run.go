// This file implements subcommands 'run' and 'print'.

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"bitbucket.org/aleist/cmdsafe/crypto"
	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type runOptions struct {
	Args     []string // Additional arguments to the saved command.
	Detached bool     // Detached mode switch.
}

// doCmdRun executes subcommand 'run' in one of two modes: if detached, it
// returns immediately after starting the child process; if not-detached, it
// waits for the child process to exit and returns the child's exit code in
// addition to any other errors.
func doCmdRun(handle string, config *runOptions) (int, error) {
	pwd, err := requestPassword(false)
	if err != nil {
		return 1, err
	}

	cmdData, err := retrieveCommandData(handle, pwd)
	if err != nil {
		return 1, err
	}

	// Append additional one-off arguments to the saved ones.
	if len(config.Args) > 0 {
		cmdData.Args = append(cmdData.Args, config.Args...)
	}

	// Run the command.
	var status int
	if config.Detached {
		if e := exec.Command(cmdData.Executable, cmdData.Args...).Start(); e != nil {
			err = fmt.Errorf("failed to start: %v", e)
		}
	} else {
		status, err = runCmd(cmdData.Executable, cmdData.Args...)
	}
	if err != nil {
		return status, fmt.Errorf("%s %v", handle, err)
	}
	return status, nil
}

// doCmdPrint executes subcommand 'print', printing the configuration of the
// command identified by handle to stdout.
func doCmdPrint(handle string) error {
	pwd, err := requestPassword(false)
	if err != nil {
		return err
	}

	cmdData, err := retrieveCommandData(handle, pwd)
	if err != nil {
		return err
	}

	fmt.Print(handle, ": ", cmdData.Executable)
	for _, arg := range cmdData.Args {
		fmt.Print(" ", arg)
	}
	fmt.Println()

	return nil
}

// retrieveCommandData loads the encrypted data stored under handle in the DB
// and attempts to decrypt it with password. Returns the decrypted data or an
// error if the password is incorrect or something else is wrong.
func retrieveCommandData(handle string, password []byte) (*Command, error) {
	// Load and parse the crypto envelope.
	cryptoEnvMsg, err := loadCommandData([]byte(handle))
	if err != nil {
		return nil, err
	}
	cryptoEnv := &crypto.CryptoEnvelope{}
	if err := proto.Unmarshal(cryptoEnvMsg, cryptoEnv); err != nil ||
		cryptoEnv.UserKey == nil || cryptoEnv.UserKey.Scrypt == nil {
		return nil, fmt.Errorf("failed to deserialise the crypto envelope: %v", err)
	}

	// Check that we support the key derivation cipher algorithms.
	if cryptoEnv.UserKey.Algorithm != crypto.KeyAlgo_SCRYPT {
		return nil, fmt.Errorf("unsupported key derivation algorithm")
	}
	if cryptoEnv.Algorithm != crypto.CipherAlgo_AES256CTR {
		return nil, fmt.Errorf("unsupported cipher algorithm")
	}

	// Derive the user key and verify its hash against the stored hash.
	scryptConfig := cryptoEnv.UserKey.Scrypt
	key, err := crypto.NewScryptKey(password, scryptConfig.Salt,
		int(scryptConfig.N), int(scryptConfig.R), int(scryptConfig.P))
	if err != nil {
		return nil, err
	}
	if bytes.Compare(key.Hash(), cryptoEnv.UserKey.Hash) != 0 {
		return nil, fmt.Errorf("incorrect password")
	}

	// Decrypt the command data.
	cmdData, err := DecryptCommand(cryptoEnv, key, crypto.DecryptAESCTR, sha256.New)
	if err != nil {
		return nil, err
	}

	// Verify that the stored command name matches the handle to ensure the DB
	// has not been tampered with and the command belongs to a different handle.
	if cmdData.Name != handle {
		return nil, fmt.Errorf("command name mismatch, the database may have been tempered with")
	}

	return cmdData, nil
}

// loadCommandData loads the unprocessed command data from key handle in the DB.
func loadCommandData(handle []byte) ([]byte, error) {
	entryNotFoundError := fmt.Errorf("%s not found", handle)

	var value []byte
	err := accessDB(true, func(db *bolt.DB) error {
		return db.View(func(tx *bolt.Tx) error {
			cmdBucket := tx.Bucket([]byte(commandBucketName))
			if cmdBucket == nil {
				return entryNotFoundError
			}

			val := cmdBucket.Get(handle)
			if val == nil {
				return entryNotFoundError
			}
			value = append(value, val...)

			return nil
		})
	})

	return value, err
}

// runCmd calls runCmdAsync and waits for the child process to complete. Listens
// for interrupts SIGINT and SIGTERM and forwards them to the child.
//
// Attempts to return the process' exit status in addition to the error if any.
func runCmd(cmdName string, arg ...string) (int, error) {
	// Disable default behaviour and pass SIGINT and SIGTERM to child process.
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)

	// Start the requested process.
	exitCh, err := runCmdAsync(interruptCh, cmdName, arg...)
	if err != nil {
		return 1, fmt.Errorf("failed to start: %v", err)
	}

	// Wait for the process to exit.
	var exitStatus int
	err = <-exitCh
	if err != nil {
		// Try to determine the exit status of the child process.
		exitStatus = 1 // Unknown reason.
		if err, ok := err.(*exec.ExitError); ok {
			if s, ok := err.Sys().(syscall.WaitStatus); ok {
				exitStatus = s.ExitStatus()
			}
		}
		err = fmt.Errorf("exited with error: %v", err)
	}
	return exitStatus, err
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
					log.Printf("Failed to forward signal to child process: %v", err)
				}
			case <-waitCh:
				done = true
			}
		}
	}()

	return exitCh, nil
}
