package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/server"
)

const (
	defaultOrgID   = "00D000000000001"
	defaultUserID  = "005000000000001"
	defaultOrgHost = "127.0.0.1"
	defaultOrgPort = 17911
)

type orgConfig struct {
	Alias       string `json:"alias"`
	Project     string `json:"project"`
	DB          string `json:"db"`
	Addr        string `json:"addr"`
	InstanceURL string `json:"instanceUrl"`
	OrgID       string `json:"orgId"`
	UserID      string `json:"userId"`
}

type orgStatus struct {
	orgConfig
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func runOrg(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || isHelpArg(args[0]) {
		return errors.New("usage: glade org create|list|status|start|auth ...")
	}
	switch args[0] {
	case "create":
		return runOrgCreate(args[1:], w)
	case "list":
		return runOrgList(args[1:], w)
	case "status":
		return runOrgStatus(ctx, args[1:], w)
	case "start":
		return runOrgStart(ctx, args[1:], w, progressW)
	case "auth":
		return runOrgAuth(ctx, args[1:], w)
	default:
		return errors.New("usage: glade org create|list|status|start|auth ...")
	}
}

func runOrgCreate(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade org create <alias> [--project <root>] [--db <path>] [--addr <host:port>] [--json]")
	}
	alias := args[0]
	if err := validateOrgAlias(alias); err != nil {
		return err
	}
	projectRoot := "."
	dbPath := ""
	addr := ""
	jsonOut := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			projectRoot = value
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if dbPath == "" {
		dbPath = filepath.Join(".glade", "orgs", alias+".sqlite")
	}
	dbOpenPath := orgDBPath(projectRoot, dbPath)
	if addr == "" {
		var err error
		addr, err = nextOrgAddr(projectRoot)
		if err != nil {
			return err
		}
	}
	store, _, err := openDBStore(dbOpenPath, projectRoot)
	if err != nil {
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}
	cfg := orgConfig{
		Alias:       alias,
		Project:     projectRoot,
		DB:          dbPath,
		Addr:        addr,
		InstanceURL: server.URL(addr),
		OrgID:       defaultOrgID,
		UserID:      defaultUserID,
	}
	if err := writeOrgConfig(projectRoot, cfg); err != nil {
		return err
	}
	if jsonOut {
		return writeOrgJSON(w, cfg)
	}
	fmt.Fprintf(w, "created org %s\n", alias)
	fmt.Fprintf(w, "Config    %s\n", filepath.ToSlash(orgConfigPath(projectRoot, alias)))
	fmt.Fprintf(w, "Database  %s\n", filepath.ToSlash(dbPath))
	fmt.Fprintf(w, "Address   %s\n", cfg.InstanceURL)
	return nil
}

func runOrgList(args []string, w io.Writer) error {
	jsonOut := false
	projectRoot := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			projectRoot = value
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	orgs, err := loadOrgConfigs(projectRoot)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeOrgJSON(w, struct {
			Orgs []orgConfig `json:"orgs"`
		}{Orgs: orgs})
	}
	if len(orgs) == 0 {
		fmt.Fprintln(w, "No local orgs configured.")
		return nil
	}
	for _, cfg := range orgs {
		fmt.Fprintf(w, "%s  %s  %s\n", cfg.Alias, cfg.InstanceURL, filepath.ToSlash(cfg.DB))
	}
	return nil
}

func runOrgStatus(ctx context.Context, args []string, w io.Writer) error {
	alias, projectRoot, jsonOut, err := parseOrgAliasProjectAndJSON("status", args)
	if err != nil {
		return err
	}
	cfg, err := readOrgConfig(projectRoot, alias)
	if err != nil {
		return err
	}
	status := checkOrgStatus(ctx, cfg)
	if jsonOut {
		return writeOrgJSON(w, status)
	}
	fmt.Fprintf(w, "%s  %s\n", status.Alias, status.Status)
	if status.Error != "" {
		fmt.Fprintf(w, "Error  %s\n", status.Error)
	}
	return nil
}

func runOrgStart(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	alias, projectRoot, err := parseOrgAliasProject("start", args)
	if err != nil {
		return err
	}
	cfg, err := readOrgConfig(projectRoot, alias)
	if err != nil {
		return err
	}
	serverArgs := []string{"--project", cfg.Project, "--db", orgDBPath(cfg.Project, cfg.DB), "--addr", cfg.Addr}
	for _, arg := range args {
		switch arg {
		case "--progress", "--progress-json", "--no-progress", "--quiet":
			serverArgs = append(serverArgs, arg)
		}
	}
	return runServer(ctx, serverArgs, w, progressW)
}

func runOrgAuth(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade org auth <alias> [--sf-config-dir <path>] [--print]")
	}
	alias := args[0]
	if err := validateOrgAlias(alias); err != nil {
		return err
	}
	projectRoot := "."
	sfConfigDir := ""
	printOnly := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			projectRoot = value
		case "--sf-config-dir":
			value, err := takeFlagValue(args, &i, "--sf-config-dir requires a path")
			if err != nil {
				return err
			}
			sfConfigDir = value
		case "--print":
			printOnly = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	cfg, err := readOrgConfig(projectRoot, alias)
	if err != nil {
		return err
	}
	token := cfg.OrgID + "!glade-local-" + cfg.Alias
	commandArgs := []string{"sf", "org", "login", "access-token", "--instance-url", cfg.InstanceURL, "--alias", cfg.Alias, "--no-prompt"}
	if printOnly {
		env := []string{shellEnvAssignment("SF_ACCESS_TOKEN", token)}
		if sfConfigDir != "" {
			env = append(env, shellEnvAssignment("SF_CONFIG_DIR", sfConfigDir))
		}
		fmt.Fprintf(w, "%s %s\n", strings.Join(env, " "), shellCommand(commandArgs...))
		return nil
	}
	status := checkOrgStatus(ctx, cfg)
	if status.Status != "running" {
		return fmt.Errorf("org %q is not running", cfg.Alias)
	}
	cmd := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Env = append(os.Environ(), "SF_ACCESS_TOKEN="+token)
	if sfConfigDir != "" {
		cmd.Env = append(cmd.Env, "SF_CONFIG_DIR="+sfConfigDir)
	}
	return cmd.Run()
}

func parseOrgAliasProject(command string, args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("usage: glade org %s <alias> [--project <root>]", command)
	}
	alias := args[0]
	if err := validateOrgAlias(alias); err != nil {
		return "", "", err
	}
	projectRoot := "."
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return "", "", err
			}
			projectRoot = value
		case "--progress", "--progress-json", "--no-progress", "--quiet":
		default:
			return "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return alias, projectRoot, nil
}

func parseOrgAliasProjectAndJSON(command string, args []string) (string, string, bool, error) {
	if len(args) == 0 {
		return "", "", false, fmt.Errorf("usage: glade org %s <alias> [--project <root>] [--json]", command)
	}
	alias := args[0]
	if err := validateOrgAlias(alias); err != nil {
		return "", "", false, err
	}
	projectRoot := "."
	jsonOut := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return "", "", false, err
			}
			projectRoot = value
		default:
			return "", "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return alias, projectRoot, jsonOut, nil
}

func validateOrgAlias(alias string) error {
	if strings.TrimSpace(alias) == "" {
		return errors.New("org alias is required")
	}
	if strings.ContainsAny(alias, `/\`) || alias == "." || alias == ".." {
		return fmt.Errorf("invalid org alias %q", alias)
	}
	return nil
}

func orgConfigPath(projectRoot, alias string) string {
	return filepath.Join(projectRoot, ".glade", "orgs", alias, "org.json")
}

func orgDBPath(projectRoot, dbPath string) string {
	if filepath.IsAbs(dbPath) {
		return dbPath
	}
	return filepath.Join(projectRoot, dbPath)
}

func writeOrgConfig(projectRoot string, cfg orgConfig) error {
	path := orgConfigPath(projectRoot, cfg.Alias)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func readOrgConfig(projectRoot, alias string) (orgConfig, error) {
	data, err := os.ReadFile(orgConfigPath(projectRoot, alias))
	if err != nil {
		return orgConfig{}, err
	}
	var cfg orgConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return orgConfig{}, err
	}
	return cfg, nil
}

func loadOrgConfigs(projectRoot string) ([]orgConfig, error) {
	root := filepath.Join(projectRoot, ".glade", "orgs")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	orgs := []orgConfig{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cfg, err := readOrgConfig(projectRoot, entry.Name())
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, cfg)
	}
	sort.Slice(orgs, func(i, j int) bool {
		return orgs[i].Alias < orgs[j].Alias
	})
	return orgs, nil
}

func nextOrgAddr(projectRoot string) (string, error) {
	orgs, err := loadOrgConfigs(projectRoot)
	if err != nil {
		return "", err
	}
	used := map[int]bool{}
	for _, cfg := range orgs {
		host, portText, err := net.SplitHostPort(cfg.Addr)
		if err != nil || host != defaultOrgHost {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err == nil {
			used[port] = true
		}
	}
	for port := defaultOrgPort; port < 65536; port++ {
		if !used[port] {
			return net.JoinHostPort(defaultOrgHost, strconv.Itoa(port)), nil
		}
	}
	return "", errors.New("no default org ports available")
}

func checkOrgStatus(ctx context.Context, cfg orgConfig) orgStatus {
	status := orgStatus{orgConfig: cfg, Status: "stopped"}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.InstanceURL, "/")+"/services/oauth2/userinfo", nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	client := http.Client{Timeout: 750 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status.Status = "running"
		return status
	}
	status.Error = resp.Status
	return status
}

func writeOrgJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func shellEnvAssignment(name, value string) string {
	return name + "=" + shellQuoteArg(value)
}
