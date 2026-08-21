package main

import "os"

type physicalFileIdentity struct {
	volume uint64
	index  uint64
}

type physicalFileIdentityResolver func(path string, info os.FileInfo) (physicalFileIdentity, error)
