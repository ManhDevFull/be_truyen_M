package utils

import (
	"crypto/sha1"
	"encoding/hex"
)

func DeviceFingerprint(userAgent, ip string) string {
	h := sha1.New()
	h.Write([]byte(userAgent))
	h.Write([]byte("|"))
	h.Write([]byte(ip))
	return hex.EncodeToString(h.Sum(nil))
}
