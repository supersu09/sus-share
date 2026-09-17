package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

var defaultPasswordHash = sync.OnceValue(func() string {
	hash, err := GenerateHash("password", DefaultArgon2Params())
	if err != nil {
		panic(err)
	}
	return hash
})

func DefaultArgon2Params() Argon2Params {
	return Argon2Params{Memory: 64 * 1024, Time: 3, Threads: 4, KeyLen: 32, SaltLen: 16}
}

func GenerateHash(password string, params Argon2Params) (string, error) {
	salt := make([]byte, params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		params.Memory, params.Time, params.Threads,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(key),
	), nil
}

func EffectivePasswordHash(configured string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	return defaultPasswordHash()
}

func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func decodeHash(encoded string) (Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id hash format")
	}
	parameterParts := strings.Split(parts[3], ",")
	if len(parameterParts) != 3 {
		return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id parameters")
	}
	var params Argon2Params
	numbered := make([]uint32, 3)
	for _, part := range parameterParts {
		key, value, found := strings.Cut(part, "=")
		if !found {
			return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id parameter")
		}
		number, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id parameter")
		}
		switch key {
		case "m":
			numbered[0] = uint32(number)
		case "t":
			numbered[1] = uint32(number)
		case "p":
			numbered[2] = uint32(number)
		default:
			return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id parameter")
		}
	}
	params.Memory, params.Time, params.Threads = numbered[0], numbered[1], uint8(numbered[2])
	salt, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id salt")
	}
	expected, err := base64.StdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 {
		return Argon2Params{}, nil, nil, errors.New("auth: invalid Argon2id hash")
	}
	params.SaltLen = uint32(len(salt))
	params.KeyLen = uint32(len(expected))
	return params, salt, expected, nil
}
