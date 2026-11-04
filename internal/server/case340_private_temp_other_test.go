//go:build !darwin && !linux

package server

func preparePrivateTestTemp() func() error { return func() error { return nil } }
