//go:build !darwin && !linux

package testdaemon

func secureDiscoveryPath(string) bool { return false }

func secureDiscoveryParentPath(string) bool { return false }

const discoveryOwnerChecks = false
