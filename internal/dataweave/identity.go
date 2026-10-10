package dataweave

import (
	"crypto/sha256"
	"encoding/hex"
)

// EngineVersion is the exact official engine version in the embedded lock.
const EngineVersion = "2.12.2"

type BuildIdentity struct {
	EngineVersion           string `json:"engineVersion"`
	ArtifactManifestSHA256  string `json:"artifactManifestSHA256"`
	AdapterSourceSHA256     string `json:"adapterSourceSHA256"`
	ApexFormatSourceSHA256  string `json:"apexFormatSourceSHA256"`
	ApexFormatServiceSHA256 string `json:"apexFormatServiceSHA256"`
	AdapterBuildKey         string `json:"adapterBuildKey"`
}

func Identity() BuildIdentity {
	digest := func(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }
	source := digest(adapterSource)
	manifest := digest(artifactManifest)
	return BuildIdentity{EngineVersion: EngineVersion, ArtifactManifestSHA256: manifest, AdapterSourceSHA256: source, ApexFormatSourceSHA256: digest(apexFormatSource), ApexFormatServiceSHA256: digest([]byte(apexFormatService)), AdapterBuildKey: digest([]byte(source + "\x00" + manifest + "\x00" + digest(apexFormatSource) + "\x00" + apexFormatService))}
}
