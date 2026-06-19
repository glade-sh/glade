package gladecli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/project"
)

type configShowInfo struct {
	ConfigPath                 string                            `json:"configPath,omitempty"`
	ConfigFound                bool                              `json:"configFound"`
	ProjectRoot                string                            `json:"projectRoot"`
	Namespace                  string                            `json:"namespace,omitempty"`
	SourceAPIVersion           string                            `json:"sourceApiVersion,omitempty"`
	PackageDirs                []string                          `json:"packageDirs"`
	OrgFeatures                []string                          `json:"orgFeatures,omitempty"`
	ManagedPackageDependencies []config.ManagedPackageDependency `json:"managedPackageDependencies,omitempty"`
	PackageShims               []config.PackageShim              `json:"packageShims,omitempty"`
}

type configInitOptions struct {
	projectRoot string
	force       bool
	yes         bool
	namespace   string
	packageDirs []string
	features    []string
}

type sfdxInitConfig struct {
	PackageDirectories []project.PackageDirectory `json:"packageDirectories"`
	Namespace          string                     `json:"namespace"`
}

func runConfig(ctxRoot string, args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade config show|validate|init")
	}
	switch args[0] {
	case "show":
		return runConfigShow(args[1:], w)
	case "validate":
		return runConfigValidate(args[1:], w)
	case "init":
		return runConfigInit(ctxRoot, args[1:], w)
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func runConfigShow(args []string, w io.Writer) error {
	root, jsonOut, err := parseProjectFlags(args)
	if err != nil {
		return err
	}
	info, err := loadConfigShowInfo(root)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Fprintf(w, "project: %s\n", info.ProjectRoot)
	if info.ConfigFound {
		fmt.Fprintf(w, "config: %s\n", info.ConfigPath)
	} else {
		fmt.Fprintln(w, "config: not found")
	}
	if info.Namespace != "" {
		fmt.Fprintf(w, "namespace: %s\n", info.Namespace)
	}
	if info.SourceAPIVersion != "" {
		fmt.Fprintf(w, "sourceApiVersion: %s\n", info.SourceAPIVersion)
	}
	fmt.Fprintf(w, "packageDirs: %s\n", strings.Join(info.PackageDirs, ", "))
	if len(info.OrgFeatures) > 0 {
		fmt.Fprintf(w, "org.features: %s\n", strings.Join(info.OrgFeatures, ", "))
	}
	if len(info.ManagedPackageDependencies) > 0 {
		fmt.Fprintf(w, "managedPackageDependencies: %d\n", len(info.ManagedPackageDependencies))
	}
	if len(info.PackageShims) > 0 {
		fmt.Fprintf(w, "packageShims: %d\n", len(info.PackageShims))
	}
	return nil
}

func runConfigValidate(args []string, w io.Writer) error {
	root := "."
	parsed, err := flagparse.New("glade config validate").
		String("project", "p").
		Parse(args)
	if err != nil {
		return err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	_, cfgPath, err := config.LoadNearest(root)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return fmt.Errorf("glade.yml not found from %s; run glade init --project %s", root, root)
		}
		return err
	}
	fmt.Fprintf(w, "config: %s\n", cfgPath)
	fmt.Fprintln(w, "status: ok")
	return nil
}

func runConfigInit(ctxRoot string, args []string, w io.Writer) error {
	opts, err := parseConfigInitOptions(args)
	if err != nil {
		return err
	}
	if opts.projectRoot == "" {
		opts.projectRoot = ctxRoot
	}
	absRoot, err := filepath.Abs(opts.projectRoot)
	if err != nil {
		return err
	}
	absRoot = filepath.Clean(absRoot)
	info, err := os.Stat(absRoot)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("project root is not a directory: %s", absRoot)
	}
	if err := fillConfigInitDefaults(absRoot, &opts); err != nil {
		return err
	}
	if !opts.yes && stdinLooksInteractive() {
		if err := promptConfigInit(os.Stdin, w, &opts); err != nil {
			return err
		}
	}
	path := filepath.Join(absRoot, "glade.yml")
	if _, err := os.Stat(path); err == nil && !opts.force {
		return fmt.Errorf("%s already exists; pass --force to overwrite", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data := renderGladeYAML(opts)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(w, "created: %s\n", path)
	fmt.Fprintf(w, "next: glade config validate --project %s\n", absRoot)
	return nil
}

func parseConfigInitOptions(args []string) (configInitOptions, error) {
	parsed, err := flagparse.New("glade config init").
		String("project", "p").
		Bool("force", "f").
		Bool("yes", "y").
		String("namespace", "n").
		String("package-dir", "").
		String("feature", "").
		Parse(args)
	if err != nil {
		return configInitOptions{}, err
	}
	return configInitOptions{
		projectRoot: parsed.String("project"),
		force:       parsed.Bool("force"),
		yes:         parsed.Bool("yes"),
		namespace:   strings.TrimSpace(parsed.String("namespace")),
		packageDirs: cleanStringList(parsed.Strings("package-dir")),
		features:    cleanStringList(parsed.Strings("feature")),
	}, nil
}

func loadConfigShowInfo(root string) (configShowInfo, error) {
	p, err := project.Load(root)
	if err != nil {
		return configShowInfo{}, err
	}
	info := configShowInfo{
		ProjectRoot:      p.Root,
		Namespace:        p.Namespace,
		SourceAPIVersion: p.SourceAPIVersion,
		PackageDirs:      packageDirNames(p.PackageDirectories),
	}
	cfg, cfgPath, err := config.LoadNearest(p.Root)
	if err == nil {
		info.ConfigFound = true
		info.ConfigPath = cfgPath
		info.OrgFeatures = cfg.Org.Features
		info.ManagedPackageDependencies = cfg.Project.ManagedPackageDependencies
		info.PackageShims = cfg.Project.PackageShims
	} else if !errors.Is(err, config.ErrNotFound) {
		return configShowInfo{}, err
	}
	return info, nil
}

func fillConfigInitDefaults(root string, opts *configInitOptions) error {
	sfdx, err := loadSFDXInitConfig(root)
	if err != nil {
		return err
	}
	if len(opts.packageDirs) == 0 {
		opts.packageDirs = packageDirNames(sfdx.PackageDirectories)
	}
	if len(opts.packageDirs) == 0 {
		opts.packageDirs = []string{"."}
	}
	if opts.namespace == "" {
		opts.namespace = strings.TrimSpace(sfdx.Namespace)
	}
	return nil
}

func promptConfigInit(in io.Reader, out io.Writer, opts *configInitOptions) error {
	reader := bufio.NewReader(in)
	packageDirs, err := promptLine(reader, out, "Package dirs", strings.Join(opts.packageDirs, ", "))
	if err != nil {
		return err
	}
	if strings.TrimSpace(packageDirs) != "" {
		opts.packageDirs = splitCommaList(packageDirs)
	}
	namespace, err := promptLine(reader, out, "Namespace", opts.namespace)
	if err != nil {
		return err
	}
	if strings.TrimSpace(namespace) != "" {
		opts.namespace = strings.TrimSpace(namespace)
	}
	features, err := promptLine(reader, out, "Org features", strings.Join(opts.features, ", "))
	if err != nil {
		return err
	}
	if strings.TrimSpace(features) != "" {
		opts.features = splitCommaList(features)
	}
	return nil
}

func promptLine(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func stdinLooksInteractive() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func loadSFDXInitConfig(root string) (sfdxInitConfig, error) {
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sfdxInitConfig{}, nil
		}
		return sfdxInitConfig{}, err
	}
	var cfg sfdxInitConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return sfdxInitConfig{}, err
	}
	return cfg, nil
}

func renderGladeYAML(opts configInitOptions) string {
	var b strings.Builder
	b.WriteString("project:\n")
	b.WriteString("  root: .\n")
	b.WriteString("  packageDirs: ")
	b.WriteString(renderInlineList(opts.packageDirs))
	b.WriteByte('\n')
	if opts.namespace != "" {
		b.WriteString("  defaultNamespace: ")
		b.WriteString(opts.namespace)
		b.WriteByte('\n')
	}
	b.WriteString("  managedPackageDependencies: []\n")
	b.WriteString("org:\n")
	b.WriteString("  features: ")
	b.WriteString(renderInlineList(opts.features))
	b.WriteByte('\n')
	return b.String()
}

func packageDirNames(dirs []project.PackageDirectory) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if value := strings.TrimSpace(filepath.ToSlash(dir.Path)); value != "" {
			out = append(out, value)
		}
	}
	return cleanStringList(out)
}

func cleanStringList(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitCommaList(value string) []string {
	return cleanStringList(strings.Split(value, ","))
}

func renderInlineList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ", ") + "]"
}
