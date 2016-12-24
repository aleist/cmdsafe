// This file implements sub-command 'save'.

package main

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type saveOptions struct {
	Replace bool // Replace existing value.
}

// doCmdSave executes sub-command 'save' for the command identified by handle.
func doCmdSave(handle string, config *saveOptions) error {
	// Serialise the Command proto.
	cmdMsg, err := proto.Marshal(cmdConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal the command config: %v", err)
	}

	// TODO encrypt
	encryptedCmd := cmdMsg

	// Write to DB.
	return accessDB(false, writeCommand([]byte(handle), encryptedCmd, config))
}

// writeCommand returns a closure that stores cmdData under handle in the DB.
func writeCommand(handle, cmdData []byte, config *saveOptions) func(*bolt.DB) error {
	return func(db *bolt.DB) error {
		if err := createBuckets(db); err != nil {
			return err
		}

		return db.Update(func(tx *bolt.Tx) error {
			cmdBucket := tx.Bucket([]byte(commandBucketName))

			// Check if entry already exists; only replace if explicitly requested.
			old := cmdBucket.Get(handle)
			if old != nil && !config.Replace {
				return fmt.Errorf("cannot replace existing entry for %s without -r flag", handle)
			}

			return cmdBucket.Put(handle, cmdData)
		})
	}
}
