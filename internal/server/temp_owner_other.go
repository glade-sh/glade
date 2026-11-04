//go:build !darwin && !linux

package server

func trustedLightningTempParent(string) bool { return false }
