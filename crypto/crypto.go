// Package crypto implements generic crypto utility functions.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"

	"golang.org/x/crypto/scrypt"
)

// Key is a cryptographic key suitable for encryption.
type Key []byte

// NewScryptKey derives two related 32-byte keys (see Encryption and HMAC) from
// password and returns them as a 64-byte Key.
//
// See golang.org/x/crypto/scrypt Key for details on the cost parameters N, r, p.
func NewScryptKey(password, salt []byte, N, r, p int) (Key, error) {
	return scrypt.Key(password, salt, N, r, p, 64)
}

// Encryption returns the first half of k, which is meant to be used with an
// encryption algorithm such as AES-256.
func (k Key) Encryption() []byte {
	l := len(k) / 2
	return k[:l:l]
}

// HMAC returns the second half of k, which is meant to be used with an HMAC
// algorithm to secure the integrity of the encrypted data.
func (k Key) HMAC() []byte {
	return k[len(k)/2:]
}

// Hash returns an SHA-256 hash of k.
func (k Key) Hash() []byte {
	sum := sha256.Sum256(k)
	return sum[:]
}

// EncryptFn is a generic cipher function as expected by Encrypt.
type EncryptFn func(key, plaintext []byte) (iv, ciphertext []byte, err error)

// DecryptFn is a generic cipher function as expected by Decrypt.
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

	// Generate a random initialization vector.
	iv = make([]byte, block.BlockSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random initialization vector: %v", err)
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

// Sign writes all given data to signer and returns the final checksum.
func Sign(signer hash.Hash, data ...[]byte) ([]byte, error) {
	for _, d := range data {
		if _, err := signer.Write(d); err != nil {
			return nil, fmt.Errorf("failed to create HMAC of crypto data: %v", err)
		}
	}
	return signer.Sum(nil), nil
}
