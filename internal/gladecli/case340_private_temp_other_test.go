//go:build !darwin && !linux

package gladecli

func preparePrivateTestTemp() func() error { return func() error { return nil } }
