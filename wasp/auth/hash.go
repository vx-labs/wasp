package auth

import (
	"crypto/sha256"
	fmt "fmt"
)

func fingerprintBytes(buf []byte) string {
	sum := sha256.Sum256(buf)
	return fmt.Sprintf("%x", sum)
}

func fingerprintString(buf string) string {
	return fingerprintBytes([]byte(buf))
}
