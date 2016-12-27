// This file implements subcommand 'save'.

package main

import (
	"crypto/sha256"
	"fmt"

	"bitbucket.org/aleist/cmdsafe/protobuf/data"
	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type saveOptions struct {
	Replace bool // Replace existing value.
}

// doCmdSave executes subcommand 'save', storing cmdData in encrypted form with
// handle as its identifier.
func doCmdSave(handle string, cmdData *data.Command, config *saveOptions) error {
	pwd, err := requestPassword(true)
	if err != nil {
		return err
	}

	// Load the global config.
	globalConfig := &data.Config{} // TODO
	scryptConfig := globalConfig.Scrypt

	// Encrypt the command data.
	key, err := NewScryptKey(pwd, scryptConfig.Salt,
		int(scryptConfig.N), int(scryptConfig.R), int(scryptConfig.P))
	if err != nil {
		return err
	}
	cryptoEnv, err := EncryptCommand(cmdData, key, EncryptAESCTR, sha256.New)
	if err != nil {
		return err
	}

	// Serialise the crypto envelope.
	cryptoEnvMsg, err := proto.Marshal(cryptoEnv)
	if err != nil {
		return fmt.Errorf("failed to serialise the crypto envelope: %v", err)
	}

	// Write to DB.
	return accessDB(false, writeCommand([]byte(handle), cryptoEnvMsg, config))
}

// writeCommand returns a closure that saves the command data value under key
// handle in the DB.
func writeCommand(handle, value []byte, config *saveOptions) func(*bolt.DB) error {
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

			return cmdBucket.Put(handle, value)
		})
	}
}
