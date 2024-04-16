package auth

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

var (
	saltChars      = "0123456789ABCDEFGHIJKLMNOPQRTSUVWXYZabcdefghijklmnopqrstuvwxyz"
	baseUrl        = "https://api.nearlyfreespeech.net"
	authHeaderName = "X-NFSN-Authentication"
	maxSalt        big.Int
)

func init() {
	maxSalt.SetInt64(int64(len(saltChars)))
}

func Salt() (string, error) {
	salt := make([]string, 16)
	for i := 0; i < 16; i++ {
		rand_index, err := rand.Int(rand.Reader, &maxSalt)
		if err != nil {
			return "", err
		}

		salt[i] = string(saltChars[rand_index.Int64()])
	}

	return strings.Join(salt, ""), nil
}

func Timestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func ComputeHash(data string) string {
	result := sha1.Sum([]byte(data))
	return hex.EncodeToString(result[:])
}

func GetAuthHeader(login string, apiKey string, url string, body string) (string, error) {
	timestamp := Timestamp()
	salt, err := Salt()
	if err != nil {
		return "", err
	}
	bodyHash := ComputeHash(body)
	hash := ComputeHash(fmt.Sprintf("%s;%s;%s;%s;%s;%s", login, timestamp, salt, apiKey, url, bodyHash))
	return fmt.Sprintf("%s;%s;%s;%s", login, timestamp, salt, hash), nil
}
