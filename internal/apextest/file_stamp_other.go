//go:build !darwin && !linux

package apextest

import "os"

// Without a trustworthy ctime every validation hashes the file.
func statFileStamp(string) (fileStamp, bool)    { return fileStamp{}, false }
func fstatFileStamp(*os.File) (fileStamp, bool) { return fileStamp{}, false }
