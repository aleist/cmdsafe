// Package crypto implements generic crypto utility functions.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"
	"strconv"

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

// Encrypt is a helper that uses function fn to encrypt plaintext.
//
// A random data encryption key is generated and itself encrypted with fn and
// userKey.Encryption. It is stored in the CryptoEnvelope.Key field with its
// initialisation vector prefixed.
//
// All public data used in the encryption process is signed with an HMAC based
// on the hash from hashFn (e.g. sha256.New) and userKey.HMAC.
func Encrypt(plaintext []byte, userKey Key, fn EncryptFn,
		hashFn func() hash.Hash) (*CryptoEnvelope, error) {

	// Currently the only supported algorithm.
	const cipherAlgo = CipherAlgo_AES256CTR

	// Generate a random encryption key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random encryption key: %v", err)
	}

	// Encrypt the data.
	iv, ciphertext, err := fn(key, plaintext)
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
	sig, err := Sign(hmac.New(hashFn, userKey.HMAC()),
		[]byte(strconv.Itoa(int(cipherAlgo))), iv, encryptedKey, ciphertext)
	if err != nil {
		return nil, err
	}

	return &CryptoEnvelope{
		Hmac:      sig,
		Iv:        iv,
		Key:       encryptedKey,
		Algorithm: cipherAlgo,
		Data:      ciphertext,
	}, nil
}

// Decrypt is a helper that uses function fn to decrypt env.Data.
//
// It verifies env.HMAC with an HMAC based on the hash from hashFn and
// userKey.HMAC. It then uses userKey.Encryption and function fn to decrypt the
// cipher key and with it the command data.
func Decrypt(env *CryptoEnvelope, userKey Key, fn DecryptFn,
		hashFn func() hash.Hash) ([]byte, error) {

	if env == nil {
		return nil, fmt.Errorf("nil crypto envelope")
	}
	if env.Algorithm != CipherAlgo_AES256CTR {
		return nil, fmt.Errorf("unsupported cipher algorithm")
	}
	const cipherBlockSize = aes.BlockSize

	// Verify the HMAC.
	sig, err := Sign(hmac.New(hashFn, userKey.HMAC()),
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
