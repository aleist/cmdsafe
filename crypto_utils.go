// This file implements cmdsafe specific crypto functionality.

package main

import (
	"fmt"
	"hash"

	"github.com/aleist/cmdsafe/crypto"
	"github.com/golang/protobuf/proto"
)

// EncryptCommand serialises cmdData and then encrypts it with crypto.Encrypt.
// See the latter for details on the parameters.
func EncryptCommand(cmdData *Command, userKey crypto.Key, fn crypto.EncryptFn,
		hashFn func() hash.Hash) (*crypto.CryptoEnvelope, error) {

	plaintext, err := proto.Marshal(cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the command data: %v", err)
	}

	return crypto.Encrypt(plaintext, userKey, fn, hashFn)
}

// DecryptCommand decrypts the Command in env.Data with crypto.Decrypt.
// See the latter for details on the parameters.
func DecryptCommand(env *crypto.CryptoEnvelope, userKey crypto.Key, fn crypto.DecryptFn,
		hashFn func() hash.Hash) (*Command, error) {

	plaintext, err := crypto.Decrypt(env, userKey, fn, hashFn)
	if err != nil {
		return nil, err
	}

	cmdData := &Command{}
	if err := proto.Unmarshal(plaintext, cmdData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the command data: %v", err)
	}

	return cmdData, nil
}
