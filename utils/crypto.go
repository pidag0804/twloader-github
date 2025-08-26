// twloader-tool/utils/crypto.go
package utils

import "encoding/base64"

func Decrypt(encodedData, key string) (string, error) {
	encrypted, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", err
	}
	keyBytes := []byte(key)
	decrypted := make([]byte, len(encrypted))
	for i := 0; i < len(encrypted); i++ {
		decrypted[i] = encrypted[i] ^ keyBytes[i%len(keyBytes)]
	}
	return string(decrypted), nil
}
