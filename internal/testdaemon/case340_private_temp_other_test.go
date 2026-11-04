//go:build !darwin && !linux

package testdaemon

func preparePrivateTestTemp() func() error { return func() error { return nil } }
