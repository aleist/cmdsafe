// This file implements subcommand 'save'.

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/aleist/cmdsafe/crypto"
	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type saveOptions struct {
	Replace bool // Replace existing value.
}

// doCmdSave executes subcommand 'save', storing cmdData in encrypted form with
// handle as its identifier.
func doCmdSave(handle string, cmdData *Command, config *saveOptions) error {
	pwd, err := requestPassword(true)
	if err != nil {
		return err
	}

	// Derive the crypto key from the password.
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate the random password salt: %v", err)
	}
	// Use default parameters from https://godoc.org/golang.org/x/crypto/scrypt
	scryptConfig := &crypto.ScryptConfig{Salt: salt, N: 16384, R: 8, P: 1}
	key, err := crypto.NewScryptKey(pwd, scryptConfig.Salt,
		int(scryptConfig.N), int(scryptConfig.R), int(scryptConfig.P))
	if err != nil {
		return err
	}

	// Encrypt the command data.
	cryptoEnv, err := EncryptCommand(cmdData, key, crypto.EncryptAESCTR, sha256.New)
	if err != nil {
		return err
	}

	// Add the user key data to the envelope.
	cryptoEnv.UserKey = &crypto.UserKey{
		Algorithm: crypto.KeyAlgo_SCRYPT,
		Hash:      key.Hash(),
		Scrypt:    scryptConfig,
	}

	// Serialise the crypto envelope.
	cryptoEnvMsg, err := proto.Marshal(cryptoEnv)
	if err != nil {
		return fmt.Errorf("failed to serialise the crypto envelope: %v", err)
	}

	// Write to DB.
	return accessDB(false, writeCommand([]byte(handle), cryptoEnvMsg, config.Replace))
}

// writeCommand returns a closure that saves the command data value under key
// handle in the DB.
func writeCommand(handle, value []byte, replace bool) func(*bolt.DB) error {
	return func(db *bolt.DB) error {
		if err := createBuckets(db); err != nil {
			return err
		}

		return db.Update(func(tx *bolt.Tx) error {
			cmdBucket := tx.Bucket([]byte(commandBucketName))

			// Check if entry already exists; only replace if explicitly requested.
			old := cmdBucket.Get(handle)
			if old != nil && !replace {
				return fmt.Errorf("cannot replace existing entry for %s without -r flag", handle)
			}

			return cmdBucket.Put(handle, value)
		})
	}
}
