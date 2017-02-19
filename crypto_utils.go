// This file implements cmdsafe specific crypto functionality.

package main

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"fmt"
	"hash"
	"strconv"

	"bitbucket.org/aleist/cmdsafe/crypto"
	"bitbucket.org/aleist/cmdsafe/protobuf/cmdsafe"
	"github.com/golang/protobuf/proto"
)

// EncryptCommand is a helper that uses function fn to encrypt cmdData.
//
// A random data encryption key is generated and itself encrypted with fn and
// userKey.Encryption. It is stored in the CryptoEnvelope.Key field with its
// initialisation vector prefixed.
//
// All public data used in the encryption process is signed with an HMAC based
// on the hash from hashFn (e.g. sha256.New) and userKey.HMAC.
func EncryptCommand(cmdData *cmdsafe.Command, userKey crypto.Key, fn crypto.EncryptFn,
	hashFn func() hash.Hash) (*crypto.CryptoEnvelope, error) {

	// Serialise the Command struct.
	cmdMsg, err := proto.Marshal(cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the command data: %v", err)
	}

	// Currently the only supported algorithm.
	const cipherAlgo = crypto.CipherAlgo_AES256CTR

	// Generate a random encryption key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random encryption key: %v", err)
	}

	// Encrypt the data.
	iv, ciphertext, err := fn(key, cmdMsg)
	if err != nil {
		return nil, err
	}

	// Encrypt the cipher key with the user key and prefix the keyIV to it.
	keyIV, keyCipher, err := fn(userKey.Encryption(), key)
	if err != nil {
		return nil, err
	}
	encryptedKey := make([]byte, 0, len(keyIV)+len(keyCipher))
	encryptedKey = append(encryptedKey, keyIV...)
	encryptedKey = append(encryptedKey, keyCipher...)

	// Use an HMAC to sign all public data used in the encryption process.
	sig, err := crypto.Sign(hmac.New(hashFn, userKey.HMAC()),
		[]byte(strconv.Itoa(int(cipherAlgo))), iv, encryptedKey, ciphertext)
	if err != nil {
		return nil, err
	}

	return &crypto.CryptoEnvelope{
		Hmac:      sig,
		Iv:        iv,
		Key:       encryptedKey,
		Algorithm: cipherAlgo,
		Data:      ciphertext,
	}, nil
}

// DecryptCommand is a helper that uses function fn to decrypt a Command.
//
// It verifies env.HMAC with an HMAC based on the hash from hashFn and
// userKey.HMAC. It then uses userKey.Encryption and function fn to decrypt the
// cipher key and with it the command data.
func DecryptCommand(env *crypto.CryptoEnvelope, userKey crypto.Key, fn crypto.DecryptFn,
	hashFn func() hash.Hash) (*cmdsafe.Command, error) {

	if env == nil {
		return nil, fmt.Errorf("nil crypto envelope")
	}
	if env.Algorithm != crypto.CipherAlgo_AES256CTR {
		return nil, fmt.Errorf("unsupported cipher algorithm")
	}
	const cipherBlockSize = aes.BlockSize

	// Verify the HMAC.
	sig, err := crypto.Sign(hmac.New(hashFn, userKey.HMAC()),
		[]byte(strconv.Itoa(int(env.Algorithm))), env.Iv, env.Key, env.Data)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(sig, env.Hmac) {
		return nil, fmt.Errorf("invalid signature, the data may have been tempered with")
	}

	// Decrypt the cipher key with the user key.
	keyIV := env.Key[:cipherBlockSize]
	keyCipher := env.Key[cipherBlockSize:]
	key, err := fn(userKey.Encryption(), keyIV, keyCipher)
	if err != nil {
		return nil, err
	}

	// Decrypt the data.
	plaintext, err := fn(key, env.Iv, env.Data)
	if err != nil {
		return nil, err
	}

	// Parse the command data.
	cmdData := &cmdsafe.Command{}
	if err := proto.Unmarshal(plaintext, cmdData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the command data: %v", err)
	}

	return cmdData, nil
}
