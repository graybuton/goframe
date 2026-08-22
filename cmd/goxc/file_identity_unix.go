//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"syscall"
)

const indexedPhysicalFileIdentity = true

func physicalFileIdentityForPath(_ string, info os.FileInfo) (physicalFileIdentity, error) {
	if info == nil {
		return physicalFileIdentity{}, fmt.Errorf("filesystem metadata is unavailable")
	}
	metadata := info.Sys()
	stat, ok := metadata.(*syscall.Stat_t)
	if !ok {
		return physicalFileIdentity{}, fmt.Errorf("filesystem metadata has type %T, want *syscall.Stat_t", metadata)
	}
	return physicalFileIdentity{
		volume: uint64(stat.Dev),
		index:  uint64(stat.Ino),
	}, nil
}
