// This file implements the crypto functionality.

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"

	"bitbucket.org/aleist/cmdsafe/protobuf/data"
	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/scrypt"
)

// CryptoKey is a cryptographic key suitable for encryption.
type CryptoKey []byte

// NewScryptKey derives two related 32-byte keys (see Encryption and HMAC) from
// password and returns them as a 64-byte CryptoKey.
//
// See golang.org/x/crypto/scrypt Key for details on the cost parameters N, r, p.
func NewScryptKey(password, salt []byte, N, r, p int) (CryptoKey, error) {
	return scrypt.Key(password, salt, N, r, p, 64)
}

// Encryption returns the first half of k, which is meant to be used with an
// encryption algorithm such as AES-256.
func (k CryptoKey) Encryption() []byte {
	l := len(k) / 2
	return k[:l:l]
}

// HMAC returns the second half of k, which is meant to be used with an HMAC
// algorithm to secure the integrity of the encrypted data.
func (k CryptoKey) HMAC() []byte {
	return k[len(k)/2:]
}

// Hash returns an SHA-256 hash of k.
func (k CryptoKey) Hash() []byte {
	sum := sha256.Sum256(k)
	return sum[:]
}

// EncryptFn is the function signature expected by EncryptCommand.
type EncryptFn func(key, plaintext []byte) (iv, ciphertext []byte, err error)

// DecryptFn is the function signature expected by DecryptCommand.
type DecryptFn func(key, iv, ciphertext []byte) (plaintext []byte, err error)

// EncryptAESCTR encrypts plaintext using the AES-CTR stream cipher.
//
// The key length determines whether AES-128, 192 or 256 is used
// (see aes.NewCipher).
//
// Returns the 16-byte initialization vector and ciphertext.
func EncryptAESCTR(key, plaintext []byte) (iv, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	// Generate a random initialization vector and prefix it to the ciphertext.
	iv = make([]byte, block.BlockSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random cipher IV: %v", err)
	}

	// Encrypt using CTR mode.
	ciphertext = make([]byte, len(plaintext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, plaintext)

	return iv, ciphertext, nil
}

// DecryptAESCTR decrypts ciphertext using the AES-CTR stream cipher with
// key and initialization vector iv and returns plaintext.
func DecryptAESCTR(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("wrong IV length, want %d, got %d", block.BlockSize(), len(iv))
	}

	// Decrypt using CTR mode.
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// EncryptCommand is a helper that uses key.Encryption and function fn to
// encrypt cmdData. It then signs all public data used in the encryption process
// with an HMAC based on the hash from hashFn (e.g. sha256.New) and key.HMAC.
//
// Returns a CryptoEnvelope with the results.
func EncryptCommand(cmdData *data.Command, key CryptoKey, fn EncryptFn,
	hashFn func() hash.Hash) (*data.CryptoEnvelope, error) {

	// Serialise the Command struct.
	cmdMsg, err := proto.Marshal(cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the command data: %v", err)
	}

	// Encrypt the data.
	iv, ciphertext, err := fn(key.Encryption(), cmdMsg)
	if err != nil {
		return nil, err
	}

	// Use an HMAC to sign all public data used in the encryption process.
	sig, err := sign(hmac.New(hashFn, key.HMAC()), iv, ciphertext)
	if err != nil {
		return nil, err
	}

	return &data.CryptoEnvelope{Hmac: sig, Iv: iv, Data: ciphertext}, nil
}

// DecryptCommand is a helper that verifies the HMAC in env with an HMAC based
// on the hash from hashFn and key.HMAC. It then uses key.Encryption and
// function fn to decrypt the command data.
//
// Returns the command config.
func DecryptCommand(env *data.CryptoEnvelope, key CryptoKey, fn DecryptFn,
	hashFn func() hash.Hash) (*data.Command, error) {

	if env == nil {
		return nil, fmt.Errorf("nil crypto envelope")
	}

	// Verify the HMAC.
	sig, err := sign(hmac.New(hashFn, key.HMAC()), env.Iv, env.Data)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(sig, env.Hmac) {
		return nil, fmt.Errorf("invalid signature, the data may have been tempered with")
	}

	// Decrypt the data.
	plaintext, err := fn(key.Encryption(), env.Iv, env.Data)
	if err != nil {
		return nil, err
	}

	// Parse the command data.
	cmdData := &data.Command{}
	if err := proto.Unmarshal(plaintext, cmdData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the command data: %v", err)
	}

	return cmdData, nil
}

// sign writes all given data to signer and returns the final checksum.
func sign(signer hash.Hash, data ...[]byte) ([]byte, error) {
	for _, d := range data {
		if _, err := signer.Write(d); err != nil {
			return nil, fmt.Errorf("failed to create HMAC of crypto data: %v", err)
		}
	}
	return signer.Sum(nil), nil
}
