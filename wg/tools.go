package wg

import (
	"encoding/base64"
	"encoding/hex"
)

// 用以解析 key 和 pubkey, 兼容 base64 和 hex 格式
func Base64ToHex(s string) string {
	if len(s) == 64 {
		return s
	}
	return hex.EncodeToString(decodeBase64(s))
}

func HexToBase64(s string) string {
	b, _ := hex.DecodeString(s)
	return encodeToBase64(b)
}

// 用以解析 key 和 pubkey, 兼容 base64 和 hex 格式
func decodeBase64(s string) []byte {
	if len(s) == 64 {
		b, _ := hex.DecodeString(s)
		return b
	}
	b, _ := base64.StdEncoding.DecodeString(s)
	return b
}
func encodeToBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
