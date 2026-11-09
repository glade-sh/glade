//go:build !darwin && !linux

package resource

func projectDirectoryIdentity(path string) string { return "" }
