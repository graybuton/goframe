//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

const indexedPhysicalFileIdentity = true

func physicalFileIdentityForPath(path string, _ os.FileInfo) (physicalFileIdentity, error) {
	pathUTF16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return physicalFileIdentity{}, fmt.Errorf("encode path: %w", err)
	}
	handle, err := syscall.CreateFile(
		pathUTF16,
		0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return physicalFileIdentity{}, fmt.Errorf("open path for identity: %w", err)
	}
	defer syscall.CloseHandle(handle)

	var identity syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &identity); err != nil {
		return physicalFileIdentity{}, fmt.Errorf("read path identity: %w", err)
	}
	return physicalFileIdentity{
		volume: uint64(identity.VolumeSerialNumber),
		index:  uint64(identity.FileIndexHigh)<<32 | uint64(identity.FileIndexLow),
	}, nil
}
