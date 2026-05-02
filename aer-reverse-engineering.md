# aer Binary: Reverse Engineering Report

## Overview

`aer` is a proprietary Salesforce Apex interpreter built by October Swimmer (`github.com/octoberswimmer`). It runs Apex code and tests locally without a Salesforce org connection. The binary is distributed as a single compiled executable with a built-in license enforcement system.

---

## Binary Characteristics

- **Path**: `~/.local/bin/aer`
- **Version**: v0.0.120
- **Format**: Mach-O arm64 executable (Apple Silicon native)
- **Size**: 132 MB
- **Language**: Go 1.25.7
- **Source module**: `github.com/octoberswimmer/aer` (private repository)
- **Build info**: Full Go symbol table preserved - all package paths, function names, and type names are readable from the binary without a debugger

The large size comes from static linking. The entire Go runtime, all dependencies, and the Apex grammar and execution engine are bundled into one file.

---

## Internal Architecture

The symbol table reveals the full internal package structure:

| Package | Purpose |
|---------|---------|
| `github.com/octoberswimmer/aer/cmd` | CLI commands, license validation |
| `github.com/octoberswimmer/aer/vm` | Apex execution engine, SObject instances, governor limits |
| `github.com/octoberswimmer/aer/storage` | SQLite-backed SObject persistence, SOQL-to-SQL transpiler |
| `github.com/octoberswimmer/aer/schema` | Salesforce schema metadata |
| `github.com/octoberswimmer/aer/driver` | Test runner, execution driver |
| `github.com/octoberswimmer/aer/discover` | Apex class discovery |
| `github.com/octoberswimmer/aer/trace` | Chrome Trace Event JSON output |
| `github.com/octoberswimmer/aer/report` | Test result reporting |
| `github.com/octoberswimmer/digger` | DataWeave interpreter (private dep, v0.24.0) |
| `github.com/octoberswimmer/apexfmt` | Apex grammar and formatter (public, BSD-licensed) |

The `apexfmt` package is the only public component. It contains the full ANTLR4 Apex grammar derived from a BSD-licensed grammar by Terence Parr and Andrey Gavrikov. This is the parse layer. Everything above it - the execution engine, SOQL transpiler, governor limits - is proprietary.

---

## License Enforcement System

### Entry Point

Every `aer` command calls `validateInstalledLicense` before doing any real work. This function:

1. Writes `0x64` (decimal 100) to the global `currentLicenseThreshold` via a store-release instruction (`stlr`)
2. Attempts to load a license key from `~/.aer/license.key` or the `AER_LICENSE_KEY` environment variable
3. If a valid license is found, overwrites `currentLicenseThreshold` with the licensed test limit
4. If no license or an invalid one, leaves the limit at 100

The test runner checks `currentLicenseThreshold` before each test. At 100 tests, it stops.

### License Key Format

A license key is a plain string in one of two forms:

```
user_<base64>
org_<base64>
```

The prefix determines the license type. A third prefix (`reg_`) is recognized but immediately rejected with the error "registration keys must be exchanged for a user license" - it exists to give a meaningful message if someone tries to use a registration/purchase receipt directly.

The base64 portion decodes to a JSON object with these fields:

```json
{
  "email": "user@example.com",
  "organization": "my-org",
  "gitHubOrganization": "gh-org",
  "publicOnly": false,
  "expires": 1735689600,
  "maxTests": 500,
  "signature": "<base64_ed25519_signature>"
}
```

All fields except `signature` are optional pointers (`*string`, `*bool`, `*int64`). The required fields depend on license type. The `signature` field is always required.

### Go Struct Layout

The JSON unmarshals into this internal struct (`licensePayloadRaw`, 56 bytes):

```go
type licensePayloadRaw struct {
    Email              *string  // +0x00
    Organization       *string  // +0x08
    GitHubOrganization *string  // +0x10
    PublicOnly         *bool    // +0x18
    Expires            *int64   // +0x20
    MaxTests           *int64   // +0x28
    Signature          *string  // +0x30
}
```

After validation and processing it becomes a `licensePayload` struct (72 bytes) with value types rather than pointers:

```go
type licensePayload struct {
    Email              string   // +0x00 (16 bytes)
    Organization       *string  // +0x10
    GitHubOrganization *string  // +0x18
    PublicOnly         *bool    // +0x20
    Expires            int64    // +0x28
    MaxTests           int64    // +0x30
    Signature          string   // +0x38 (16 bytes)
}
```

---

## Cryptographic Signature Verification

### Algorithm

Ed25519 (`crypto/ed25519.VerifyWithOptions` from the Go standard library). This is a modern elliptic-curve signature scheme - fast, compact, and secure. There is no symmetric encryption involved anywhere in the license system.

### Embedded Public Key

The 32-byte Ed25519 public key is stored in the BSS segment at vmaddr `0x105af55d0` and initialized at startup by `cmd.init` via `mustDecodePublicKey`:

```
Base64:  LIOz1s+Ts/Nu7js2xNijt5YakgDXsPEGWFYflxB35bw=
Hex:     2c83b3d6cf93b3f36eee3b36c4d8a3b7961a9200d7b0f10658561f971077e5bc
```

The corresponding private key never appears in the binary. Only October Swimmer holds it. Valid signatures cannot be generated without it.

### Signed Message Construction

The message that gets signed is built by joining specific fields with `\n` (newline), then passed to `ed25519.VerifyWithOptions` as bytes.

**For `org_` licenses:**
```
email\norganization\nFormatInt(expires, 10)\nFormatInt(maxTests, 10)
```

**For `user_` licenses:**
```
[gitHubOrganization\n][FormatInt(expires, 10)\n][FormatInt(maxTests, 10)][\npublic-only]
```

The `gitHubOrganization` and `public-only` parts are only included if those optional fields are present in the payload.

The `signature` field in the JSON is the Ed25519 signature of that message, base64-encoded, produced with the private key that corresponds to the embedded public key.

---

## Validation Call Chain

```
validateInstalledLicense
  └─ loadLicenseKey (reads file or AER_LICENSE_KEY env var)
  └─ validateLicenseKey(keyString, now, loc)
       └─ decodeLicensePayload(keyString)
            └─ parseLicenseKey(keyString)
                 - TrimSpace
                 - Check prefix: "user_" -> type 1, "org_" -> type 2, "reg_" -> type 3 error
                 - Strip prefix
            └─ base64.StdEncoding.DecodeString(keyBody)
            └─ json.Unmarshal(bytes, &licensePayloadRaw{})
            └─ validate required fields present
            └─ return *licensePayload
       - Check expires > now (time.Time.After)
       - Build signed message string (strings.Join with "\n")
       - base64.StdEncoding.DecodeString(payload.Signature)
       - ed25519.VerifyWithOptions(pubkey, message, signature)
       - [org licenses] verifyGitHubOrgMembership(org, isPublicOnly, githubToken)
       - [org licenses, async] maybeReportOrgLicenseUsage(...)
```

---

## Network Activity

License validation is not purely local in all cases:

| Condition | Network call | Endpoint |
|-----------|-------------|----------|
| `org_` license | `verifyGitHubOrgMembership` | GitHub API (`api.github.com`) |
| `user_` license with `publicOnly=true` | Same GitHub org check | GitHub API |
| `user_` license without `publicOnly` | None during validation | - |
| `org_` license (async, post-validation) | `maybeReportOrgLicenseUsage` | `https://kingdom.octoberswimmer.com/` |
| Dev license near expiry | `maybeStartDevLicenseRenewal` | `https://kingdom.octoberswimmer.com/` |

A plain `user_` dev license without `publicOnly` validates entirely offline. The telemetry and renewal calls are async and non-blocking.

The `GITHUB_TOKEN` environment variable is required for org licenses and public-only user licenses. It is used to call the GitHub API to verify org membership.

---

## The 100-Test Free Tier

The free tier limit works as follows:

1. `validateInstalledLicense` always writes `100` to `currentLicenseThreshold` first (unconditional `stlr` at `0x102a9c818`)
2. If a valid license is found with `maxTests > 0`, it overwrites with `maxTests`
3. If `maxTests = -1` (`0x7fffffffffffffff`), the value written is `0x7fffffffffffffff` (effectively unlimited)
4. The test runner checks this global before each test

There is no grace period or run-level cap - it is a strict per-run limit of 100 tests.

---

## CI Detection

Dev licenses (`user_` prefix) check for CI environment variables before allowing execution. The binary searches a list of known CI env vars (`CI`, `GITHUB_ACTIONS`, `CIRCLECI`, etc.) using `os.LookupEnv`. If any are set, `validateLicenseKey` returns the error "developer licenses cannot be used in CI environment %s". Org licenses do not have this restriction.

---

## Key Storage

By default the license key is read from `~/.aer/license.key`. The `AER_LICENSE_KEY` environment variable overrides this, allowing the key to be passed via secrets in CI/CD pipelines (with an org license).

---

## Public Components Available for Reuse

`github.com/octoberswimmer/apexfmt` (v0.52.0, BSD-licensed) contains:

- `grammar/ApexLexer.g4` and `grammar/ApexParser.g4` - the full Apex ANTLR4 grammar
- `parser/apex_parser.go` (1.1 MB generated) and `parser/apex_lexer.go` (124 KB generated)
- Dependencies: `antlr4-go/antlr v4.13.0`

This is the complete parse layer. It produces a full AST for any Apex source file. The execution engine, SOQL transpiler, governor limit tracking, and SObject persistence are all proprietary and not available separately.

---

## Summary

The license system uses standard, well-implemented cryptography (Ed25519) with the private key held exclusively by October Swimmer. There is no hardware binding, no machine fingerprinting, and no online activation step for dev licenses. The free tier enforces 100 tests per run by writing a global at startup. A valid license key overwrites that global with a higher limit. The key itself is a portable string that works on any machine.
