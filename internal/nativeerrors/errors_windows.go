// Package nativeerrors converts Windows driver error strings before JSON encoding.
package nativeerrors

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

type decodedError struct {
	cause   error
	message string
}

func (e decodedError) Error() string { return e.message }
func (e decodedError) Unwrap() error { return e.cause }

func Normalize(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	// Npcap appends the decimal Windows status even when its localized text
	// uses the system ANSI code page. ERROR_NDIS_MEDIA_DISCONNECTED is 0x8034001F.
	if strings.Contains(message, "(2150891551)") || strings.Contains(strings.ToLower(message), "0x8034001f") {
		return decodedError{err, "网卡连接已断开，请检查网线、网卡是否启用，并刷新后选择已连接的有线网卡"}
	}
	if !utf8.ValidString(message) {
		message = decodeCodePage(message, 0) // CP_ACP: the current Windows ANSI code page.
	}
	return decodedError{err, strings.ToValidUTF8(message, "?")}
}

func decodeCodePage(value string, codePage uint32) string {
	if value == "" {
		return value
	}
	data := []byte(value)
	wide := make([]uint16, len(data))
	count, err := windows.MultiByteToWideChar(codePage, 0, &data[0], int32(len(data)), &wide[0], int32(len(wide)))
	if err != nil {
		return strings.ToValidUTF8(value, "?")
	}
	return windows.UTF16ToString(wide[:count])
}
