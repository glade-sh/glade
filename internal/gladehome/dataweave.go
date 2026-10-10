package gladehome

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dataweave"
)

type dataWeaveInstallation struct {
	AdapterKey   string                  `json:"adapterKey"`
	Version      int                     `json:"version"`
	Identity     dataweave.BuildIdentity `json:"identity"`
	JavaHome     string                  `json:"javaHome"`
	JavaPath     string                  `json:"javaPath"`
	JavacPath    string                  `json:"javacPath"`
	JavaSHA256   string                  `json:"javaSHA256"`
	JavacSHA256  string                  `json:"javacSHA256"`
	JavaVersion  string                  `json:"javaVersion"`
	JavacVersion string                  `json:"javacVersion"`
	AdapterFiles map[string]string       `json:"adapterFiles"`
}

// DataWeaveToolchainStatus describes a verified installation without downloading.
type DataWeaveToolchainStatus struct {
	OK              bool   `json:"ok"`
	Path            string `json:"path"`
	Detail          string `json:"detail"`
	EngineVersion   string `json:"engineVersion"`
	JavaPath        string `json:"javaPath,omitempty"`
	JavaVersion     string `json:"javaVersion,omitempty"`
	AdapterBuildKey string `json:"adapterBuildKey,omitempty"`
}

func dataWeaveDirectory() string {
	root := ""
	for _, name := range []string{"GLADE_HOME", "GLADE_ROOT"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			root = value
			break
		}
	}
	if root == "" {
		root = UserShareDir()
	}
	return filepath.Join(root, "toolchains", "dataweave", dataweave.EngineVersion)
}

// InstallDataWeave explicitly provisions the pinned engine and compiles the owned
// adapter using a caller-selected Java17 JDK. No Java distribution is downloaded.
func InstallDataWeave(ctx context.Context, javaHome string) (DataWeaveToolchainStatus, error) {
	root, err := filepath.Abs(dataWeaveDirectory())
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	if strings.TrimSpace(javaHome) == "" {
		return DataWeaveToolchainStatus{}, fmt.Errorf("DataWeave installation requires --java-home pointing to a Java17 JDK")
	}
	javaHome, err = filepath.Abs(javaHome)
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	javaHome, err = filepath.EvalSymlinks(javaHome)
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	java := filepath.Join(javaHome, "bin", "java"+suffix)
	javac := filepath.Join(javaHome, "bin", "javac"+suffix)
	javaVersion, err := java17Version(ctx, java, "-version")
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	javacVersion, err := java17Version(ctx, javac, "-version")
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	javaHash, err := dataWeaveFileHash(java)
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	javacHash, err := dataWeaveFileHash(javac)
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	identity := dataweave.Identity()
	adapterKey := dataWeaveAdapterKey(identity, javacHash)
	classPath, err := dataweave.InstallEngine(ctx, filepath.Join(root, "engine"))
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	adapter := filepath.Join(root, "adapters", adapterKey)
	if err := os.MkdirAll(filepath.Dir(adapter), 0700); err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(adapter), ".compile-")
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	defer os.RemoveAll(stage)
	compileCtx, cancelCompile := context.WithTimeout(ctx, 30*time.Second)
	defer cancelCompile()
	if err := dataweave.CompileAdapter(compileCtx, javac, classPath, stage); err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	files, err := dataWeaveAdapterFiles(stage)
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	if _, err := os.Stat(adapter); os.IsNotExist(err) {
		if err := os.Rename(stage, adapter); err != nil {
			return DataWeaveToolchainStatus{}, err
		}
	} else if err != nil {
		return DataWeaveToolchainStatus{}, err
	} else {
		if err := verifyDataWeaveAdapter(adapter, files); err != nil {
			return DataWeaveToolchainStatus{}, fmt.Errorf("existing DataWeave adapter differs from selected JDK output: %w", err)
		}
	}
	manifest := dataWeaveInstallation{AdapterKey: adapterKey, Version: 1, Identity: identity, JavaHome: javaHome, JavaPath: java, JavacPath: javac, JavaSHA256: javaHash, JavacSHA256: javacHash, JavaVersion: javaVersion, JavacVersion: javacVersion, AdapterFiles: files}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	tmp, err := os.CreateTemp(root, ".installation-")
	if err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(payload, '\n')); err != nil {
		tmp.Close()
		return DataWeaveToolchainStatus{}, err
	}
	if err := tmp.Close(); err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(root, "installation.json")); err != nil {
		return DataWeaveToolchainStatus{}, err
	}
	status := DataWeaveStatus()
	if !status.OK {
		return status, fmt.Errorf("verify installed DataWeave component: %s", status.Detail)
	}
	return status, nil
}

// DataWeaveRuntime loads a previously installed and verified runtime. It never
// installs dependencies or modifies the toolchain during execution.
func DataWeaveRuntime() (dataweave.Runtime, error) {
	root, manifest, err := loadDataWeaveInstallation()
	if err != nil {
		return dataweave.Runtime{}, err
	}
	classPath, err := dataweave.VerifyEngine(filepath.Join(root, "engine"))
	if err != nil {
		return dataweave.Runtime{}, err
	}
	return dataweave.Runtime{JavaPath: manifest.JavaPath, ClassPath: classPath, AdapterDirectory: filepath.Join(root, "adapters", manifest.AdapterKey)}, nil
}
func DataWeaveStatus() DataWeaveToolchainStatus {
	root, err := filepath.Abs(dataWeaveDirectory())
	if err != nil {
		return DataWeaveToolchainStatus{Detail: err.Error()}
	}
	status := DataWeaveToolchainStatus{Path: root, EngineVersion: dataweave.EngineVersion}
	_, manifest, err := loadDataWeaveInstallation()
	if err != nil {
		status.Detail = err.Error()
		return status
	}
	if _, err := dataweave.VerifyEngine(filepath.Join(root, "engine")); err != nil {
		status.Detail = err.Error()
		return status
	}
	status.OK = true
	status.Detail = "verified"
	status.JavaPath = manifest.JavaPath
	status.JavaVersion = manifest.JavaVersion
	status.AdapterBuildKey = manifest.AdapterKey
	return status
}
func loadDataWeaveInstallation() (string, dataWeaveInstallation, error) {
	root, err := filepath.Abs(dataWeaveDirectory())
	if err != nil {
		return "", dataWeaveInstallation{}, err
	}
	path := filepath.Join(root, "installation.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return root, dataWeaveInstallation{}, fmt.Errorf("DataWeave toolchain unavailable; run glade toolchain install dataweave --java-home <Java17-JDK>: %w", err)
	}
	var manifest dataWeaveInstallation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return root, manifest, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return root, manifest, fmt.Errorf("DataWeave installation manifest has trailing data")
	}

	if manifest.Version != 1 || manifest.Identity != dataweave.Identity() || manifest.AdapterKey != dataWeaveAdapterKey(manifest.Identity, manifest.JavacSHA256) {
		return root, manifest, fmt.Errorf("DataWeave installation identity differs from this Glade build; reinstall component")
	}
	if !filepath.IsAbs(manifest.JavaHome) || !filepath.IsAbs(manifest.JavaPath) || !filepath.IsAbs(manifest.JavacPath) {
		return root, manifest, fmt.Errorf("DataWeave Java toolchain paths must be absolute")
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	if manifest.JavaPath != filepath.Join(manifest.JavaHome, "bin", "java"+suffix) || manifest.JavacPath != filepath.Join(manifest.JavaHome, "bin", "javac"+suffix) || !java17Pattern.MatchString(manifest.JavaVersion) || !java17Pattern.MatchString(manifest.JavacVersion) {
		return root, manifest, fmt.Errorf("invalid DataWeave Java17 installation identity")
	}

	for path, want := range map[string]string{manifest.JavaPath: manifest.JavaSHA256, manifest.JavacPath: manifest.JavacSHA256} {
		got, err := dataWeaveFileHash(path)
		if err != nil {
			return root, manifest, err
		}
		if got != want {
			return root, manifest, fmt.Errorf("DataWeave Java executable changed: %s", path)
		}
	}
	if err := verifyDataWeaveAdapter(filepath.Join(root, "adapters", manifest.AdapterKey), manifest.AdapterFiles); err != nil {
		return root, manifest, err
	}
	return root, manifest, nil
}

var java17Pattern = regexp.MustCompile(`(?m)^(?:openjdk|java|javac)(?: version)? "?17(?:[. +"\r\n]|$)`)

func java17Version(ctx context.Context, path, flag string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, flag)
	cmd.Env = []string{"LANG=C.UTF-8"}
	if root := os.Getenv("SystemRoot"); root != "" {
		cmd.Env = append(cmd.Env, "SystemRoot="+root)
	}
	var output dataWeaveCommandOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("inspect Java17 tool %s: %w", path, err)
	}
	value := strings.TrimSpace(output.String())
	if output.exceeded || !java17Pattern.MatchString(value) {
		return "", fmt.Errorf("DataWeave requires Java17 tools; %s reported %q", path, value)
	}
	return value, nil
}

type dataWeaveCommandOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *dataWeaveCommandOutput) Write(data []byte) (int, error) {
	remaining := (16 << 10) - b.Len()
	if len(data) > remaining {
		b.exceeded = true
		if remaining > 0 {
			b.Buffer.Write(data[:remaining])
		}
		return len(data), nil
	}
	return b.Buffer.Write(data)
}
func dataWeaveFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func dataWeaveAdapterFiles(directory string) (map[string]string, error) {
	files := map[string]string{}
	const descriptor = "META-INF/services/org.mule.weave.v2.module.DataFormat"
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "." && relative != "META-INF" && relative != "META-INF/services" {
				return fmt.Errorf("unexpected DataWeave adapter directory: %s", relative)
			}
			return nil
		}
		if strings.Contains(relative, "/") && relative != descriptor {
			return fmt.Errorf("unexpected DataWeave adapter resource: %s", relative)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("DataWeave adapter cannot contain symbolic links: %s", relative)
		}
		hash, err := dataWeaveFileHash(path)
		if err != nil {
			return err
		}
		files[relative] = hash
		return nil
	})
	if err != nil {
		return nil, err
	}
	identity := dataweave.Identity()
	if files["Adapter.java"] != identity.AdapterSourceSHA256 || files["OwnedApexFormat.java"] != identity.ApexFormatSourceSHA256 || files[descriptor] != identity.ApexFormatServiceSHA256 || files["Adapter.class"] == "" || files["OwnedApexFormat.class"] == "" {
		return nil, fmt.Errorf("DataWeave adapter source, class or exact format descriptor missing")
	}
	return files, nil
}
func verifyDataWeaveAdapter(directory string, want map[string]string) error {
	actual, err := dataWeaveAdapterFiles(directory)
	if err != nil {
		return err
	}
	if len(actual) != len(want) {
		return fmt.Errorf("DataWeave adapter file inventory changed")
	}
	for name, hash := range actual {
		if hash != want[name] {
			return fmt.Errorf("DataWeave adapter file changed: %s", name)
		}
	}
	return nil
}

func dataWeaveAdapterKey(identity dataweave.BuildIdentity, javacSHA string) string {
	sum := sha256.Sum256([]byte(identity.AdapterBuildKey + "\x00" + javacSHA))
	return hex.EncodeToString(sum[:])
}
