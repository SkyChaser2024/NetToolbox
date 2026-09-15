package auth

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	eapolVersion = 1
	eapolEAP     = 0
	eapolStart   = 1
	eapolLogoff  = 2
	maxEAPLength = 1<<16 - 1
)

// DecodeHex accepts the separators commonly used when documenting vendor
// extensions, while keeping the bytes sent on the wire deterministic.
func DecodeHex(value string) ([]byte, error) {
	cleaned := strings.NewReplacer(" ", "", ":", "", "-", "").Replace(value)
	if len(cleaned)%2 != 0 {
		return nil, errors.New("十六进制扩展的字符数必须为偶数")
	}
	if len(cleaned)/2 > maxEAPLength-5 {
		return nil, errors.New("十六进制扩展数据过长")
	}
	return hex.DecodeString(cleaned)
}

func BuildIdentity(identity, suffixHex string) ([]byte, error) {
	suffix, err := DecodeHex(suffixHex)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, len(identity)+len(suffix))
	result = append(result, []byte(identity)...)
	result = append(result, suffix...)
	if len(result) > maxEAPLength-5 {
		return nil, errors.New("identity 与扩展数据过长")
	}
	return result, nil
}

func BuildIdentityResponse(identifier byte, identity []byte) ([]byte, error) {
	eapLength := 5 + len(identity)
	if eapLength > maxEAPLength {
		return nil, errors.New("identity 响应超过 EAP 报文长度限制")
	}
	eap := make([]byte, eapLength)
	eap[0] = 2 // Response
	eap[1] = identifier
	eap[2] = byte(eapLength >> 8)
	eap[3] = byte(eapLength)
	eap[4] = 1 // Identity
	copy(eap[5:], identity)
	return eap, nil
}

func BuildMD5Response(identifier byte, password string, challenge, username []byte) ([]byte, error) {
	if len(challenge) == 0 {
		return nil, errors.New("MD5 Challenge 为空")
	}
	eapLength := 5 + 1 + md5.Size + len(username)
	if eapLength > maxEAPLength {
		return nil, fmt.Errorf("用户名过长，EAP-MD5 响应需要 %d 字节", eapLength)
	}
	input := make([]byte, 1+len(password)+len(challenge))
	input[0] = identifier
	copy(input[1:], password)
	copy(input[1+len(password):], challenge)
	defer clear(input)
	digest := md5.Sum(input) // EAP-MD5 is specified by RFC 3748 and requires MD5.

	typeData := make([]byte, 1+len(digest)+len(username))
	typeData[0] = byte(len(digest))
	copy(typeData[1:17], digest[:])
	copy(typeData[17:], username)

	eap := make([]byte, eapLength)
	eap[0] = 2 // Response
	eap[1] = identifier
	eap[2] = byte(eapLength >> 8)
	eap[3] = byte(eapLength)
	eap[4] = 4 // MD5-Challenge
	copy(eap[5:], typeData)
	clear(typeData)
	clear(digest[:])
	return eap, nil
}

func buildEAPOL(packetType byte, eap []byte) []byte {
	payload := make([]byte, 4+len(eap))
	payload[0] = eapolVersion
	payload[1] = packetType
	payload[2] = byte(len(eap) >> 8)
	payload[3] = byte(len(eap))
	copy(payload[4:], eap)
	return payload
}
