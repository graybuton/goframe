//go:build !linux && !darwin && !windows

package main

import (
	"errors"
	"os"
)

const indexedPhysicalFileIdentity = false

func physicalFileIdentityForPath(_ string, _ os.FileInfo) (physicalFileIdentity, error) {
	return physicalFileIdentity{}, errors.New("indexed physical file identity is unavailable on this host")
}
