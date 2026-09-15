//go:build windows

package settings

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dataBlob struct {
	size uint32
	data *byte
}

var (
	crypt32            = windows.NewLazySystemDLL("crypt32.dll")
	cryptProtectData   = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	passwordEntropy    = []byte("CampusNetToolbox/password/v1")
)

const cryptProtectUIForbidden = 0x1

func blob(value []byte) (dataBlob, error) {
	if uint64(len(value)) > math.MaxUint32 {
		return dataBlob{}, errors.New("DPAPI 输入数据过大")
	}
	if len(value) == 0 {
		return dataBlob{}, nil
	}
	return dataBlob{size: uint32(len(value)), data: &value[0]}, nil
}

func protect(plain []byte) ([]byte, error) {
	in, err := blob(plain)
	if err != nil {
		return nil, err
	}
	entropy, err := blob(passwordEntropy)
	if err != nil {
		return nil, err
	}
	var out dataBlob
	result, _, callErr := cryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, uintptr(unsafe.Pointer(&entropy)), 0, 0,
		cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(plain)
	runtime.KeepAlive(passwordEntropy)
	if result == 0 {
		return nil, fmt.Errorf("使用 Windows DPAPI 加密失败: %w", callErr)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.data))))
	return append([]byte(nil), unsafe.Slice(out.data, out.size)...), nil
}

func unprotect(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("密码数据为空")
	}
	in, err := blob(ciphertext)
	if err != nil {
		return nil, err
	}
	entropy, err := blob(passwordEntropy)
	if err != nil {
		return nil, err
	}
	var out dataBlob
	result, _, callErr := cryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, uintptr(unsafe.Pointer(&entropy)), 0, 0,
		cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(ciphertext)
	runtime.KeepAlive(passwordEntropy)
	if result == 0 {
		return nil, fmt.Errorf("使用 Windows DPAPI 解密失败: %w", callErr)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.data))))
	return append([]byte(nil), unsafe.Slice(out.data, out.size)...), nil
}
