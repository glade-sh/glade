package cliui

import (
	"fmt"
	"io"
	"strings"
)

type HelpCommand struct {
	Name        string
	Description string
}

type FlagHelp struct {
	Name        string
	Value       string
	Description string
}

func (f FlagHelp) Display() string {
	if f.Value == "" {
		return f.Name
	}
	return f.Name + " " + f.Value
}

type SubcommandHelp struct {
	Name        string
	Description string
}

type CommandHelp struct {
	Name        string
	Description string
	Usage       []string
	Subcommands []SubcommandHelp
	Flags       []FlagHelp
	Examples    []string
}

var commandReferences = []CommandHelp{
	{
		Name:        "version",
		Description: "Print the glade version.",
		Usage:       []string{"glade version [--json]"},
		Flags:       []FlagHelp{{Name: "--json", Description: "Write version, Go runtime, OS, and architecture as JSON."}},
		Examples:    []string{"glade version", "glade version --json"},
	},
	{
		Name:        "doctor",
		Description: "Print environment and project configuration status.",
		Usage:       []string{"glade doctor [--project <root>] [--json]"},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--json", Description: "Write doctor status as JSON."},
		},
		Examples: []string{"glade doctor", "glade doctor --project . --json"},
	},
	{
		Name:        "toolchain",
		Description: "Install or inspect the global LWC toolchain for Lightning Out.",
		Usage:       []string{"glade toolchain install [--from <glade-checkout>]", "glade toolchain status"},
		Subcommands: []SubcommandHelp{
			{Name: "install", Description: "Install the LWC runtime toolchain."},
			{Name: "status", Description: "Print the current LWC toolchain status."},
		},
		Flags: []FlagHelp{
			{Name: "--from", Value: "<glade-checkout>", Description: "Install from another glade checkout. Defaults to the current checkout."},
		},
		Examples: []string{"glade toolchain status", "glade toolchain install --from ."},
	},
	{
		Name:        "config",
		Description: "Inspect, validate, and create glade.yml.",
		Usage:       []string{"glade config show [--project <root>] [--json]", "glade config validate [--project <root>]", "glade config init [--project <root>] [--yes] [--force] [--namespace <name>] [--package-dir <path>] [--feature <name>]"},
		Subcommands: []SubcommandHelp{
			{Name: "show", Description: "Print resolved glade.yml and SFDX project settings."},
			{Name: "validate", Description: "Validate glade.yml syntax and supported keys."},
			{Name: "init", Description: "Create a glade.yml starter file."},
		},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--json", Description: "Write config show output as JSON."},
			{Name: "--yes", Description: "Accept inferred defaults when creating glade.yml."},
			{Name: "--force", Description: "Overwrite an existing glade.yml."},
			{Name: "--namespace", Value: "<name>", Description: "Default package namespace."},
			{Name: "--package-dir", Value: "<path>", Description: "Package directory. Repeat for more than one."},
			{Name: "--feature", Value: "<name>", Description: "Org feature. Repeat for more than one."},
		},
		Examples: []string{"glade config show --project .", "glade config validate --project .", "glade config init --project . --yes"},
	},
	{
		Name:        "init",
		Description: "Create a glade.yml starter file.",
		Usage:       []string{"glade init [--project <root>] [--yes] [--force] [--namespace <name>] [--package-dir <path>] [--feature <name>]"},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--yes", Description: "Accept inferred defaults."},
			{Name: "--force", Description: "Overwrite an existing glade.yml."},
			{Name: "--namespace", Value: "<name>", Description: "Default package namespace."},
			{Name: "--package-dir", Value: "<path>", Description: "Package directory. Repeat for more than one."},
			{Name: "--feature", Value: "<name>", Description: "Org feature. Repeat for more than one."},
		},
		Examples: []string{"glade init --project . --yes", "glade init --namespace pkg --package-dir force-app"},
	},
	{
		Name:        "parse",
		Description: "Parse Apex source files.",
		Usage:       []string{"glade parse <paths...> [--json] [--progress|--progress-json|--no-progress]"},
		Flags: []FlagHelp{
			{Name: "--json", Description: "Write parsed files and diagnostics as JSON."},
			{Name: "--progress", Description: "Print line progress to stderr."},
			{Name: "--progress-json", Description: "Print NDJSON progress events to stderr."},
			{Name: "--no-progress", Description: "Disable terminal progress."},
		},
		Examples: []string{"glade parse force-app/main/default/classes/AccountService.cls", "glade parse force-app --progress", "glade parse force-app --json"},
	},
	{
		Name:        "inspect",
		Description: "Inspect indexed project symbols.",
		Usage:       []string{"glade inspect symbols [--project <root>] [--json]", "glade inspect graph [--project <root>] [--json]"},
		Subcommands: []SubcommandHelp{
			{Name: "symbols", Description: "Print indexed Apex, trigger, and metadata symbols."},
			{Name: "graph", Description: "Print the enterprise project graph."},
		},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--json", Description: "Write structured output."},
		},
		Examples: []string{"glade inspect symbols --project .", "glade inspect graph --project . --json"},
	},
	{
		Name:        "schema",
		Description: "Load local Salesforce metadata schema.",
		Usage: []string{
			"glade schema load [--project <root>] [--json] [--progress|--progress-json|--no-progress]",
			"glade schema import describe --input <describe.json> [--output <schema.json>]",
		},
		Subcommands: []SubcommandHelp{
			{Name: "load", Description: "Load and print local schema information."},
			{Name: "import describe", Description: "Import captured Salesforce describe JSON into a local Glade schema."},
		},
		Flags: projectProgressFlags("Write schema as JSON."),
		Examples: []string{
			"glade schema load --project .",
			"glade schema load --project . --progress",
			"glade schema import describe --input reports/org-describe.json --output schema/local.schema.json",
		},
	},
	{
		Name:        "check",
		Description: "Run semantic checks over a project.",
		Usage:       []string{"glade check [--project <root>] [--format text|json|sarif|github] [--output <path>] [--progress|--progress-json|--no-progress]"},
		Flags: append(projectProgressFlags("Write semantic result as JSON."),
			FlagHelp{Name: "--format", Value: "<mode>", Description: "Output format: text, json, sarif, or github."},
			FlagHelp{Name: "--output", Value: "<path>", Description: "Write the selected output format to a file."},
		),
		Examples: []string{"glade check --project .", "glade check --project . --format sarif --output glade.sarif", "glade check --project . --format github"},
	},
	{
		Name:        "exec",
		Description: "Execute anonymous Apex.",
		Usage:       []string{"glade exec [--project <root>] [--db <path>] [--dry-run] [--json] [--trace <path>] [--debug-log <path>] [--limit-mode <mode>] '<anonymous apex>'"},
		Flags: []FlagHelp{
			{Name: "--project, -p", Value: "<root>", Description: "SFDX project root used for metadata and local org shape."},
			{Name: "--db", Value: "<path>", Description: "SQLite local org path for DB-backed anonymous Apex execution."},
			{Name: "--dry-run", Description: "Run against the selected local org without saving changes."},
			{Name: "--json", Description: "Write VM result and trace as JSON."},
			{Name: "--debug", Description: "Serve one DAP snapshot for the run."},
			{Name: "--trace", Value: "<path>", Description: "Write trace JSON to a file."},
			{Name: "--debug-log", Value: "<summary|raw|path>", Description: "Select debug-log mode or write a Salesforce-style debug log. Use - for stdout."},
			{Name: "--log-out", Value: "<path>", Description: "Write the raw Salesforce-style debug log to a file."},
			{Name: "--limit-mode", Value: "<mode>", Description: "Governor limit mode: permissive or strict."},
		},
		Examples: []string{`glade exec "System.debug('hello');"`, `glade exec --debug-log raw --log-out reports/exec.log "System.debug('hello');"`, `glade exec --json "Integer x = 1;"`, `glade exec --project . --db .glade/envs/dev.sqlite "insert new Account(Name = 'Local');"`},
	},
	{
		Name:        "debug",
		Description: "Parse, profile, explain, and synthesize from Salesforce debug logs.",
		Usage:       []string{"glade debug parse --log <path> [--json]", "glade debug profile --log <path> [--json] [--format text|markdown]", "glade debug explain --log <path> [--project <root>] [--min-confidence <n>] [--json]", "glade debug repro --log <path> [--project <root>] [--min-confidence <n>]"},
		Subcommands: []SubcommandHelp{
			{Name: "parse", Description: "Parse a Salesforce debug log."},
			{Name: "profile", Description: "Profile a parsed debug log."},
			{Name: "explain", Description: "Annotate log frames with project symbols."},
			{Name: "repro", Description: "Synthesize a local test from a debug log."},
		},
		Flags: []FlagHelp{
			{Name: "--log", Value: "<path>", Description: "Debug log path. Use - where supported."},
			{Name: "--project", Value: "<root>", Description: "Project root for explain and repro."},
			{Name: "--min-confidence", Value: "<n>", Description: "Minimum confidence for explain and repro."},
			{Name: "--json", Description: "Write structured output where available."},
			{Name: "--format", Value: "<mode>", Description: "Profile output format: text or markdown."},
		},
		Examples: []string{"glade debug profile --log apex.log", "glade debug explain --log apex.log --project ."},
	},
	{
		Name:        "editor",
		Description: "Install and check editor integrations.",
		Usage:       []string{"glade editor install vscode [--vsix <path>] [--editor <code|cursor|windsurf>] [--force]", "glade editor doctor vscode [--editor <code|cursor|windsurf>] [--json]"},
		Subcommands: []SubcommandHelp{
			{Name: "install", Description: "Install an editor extension package."},
			{Name: "doctor", Description: "Check editor and glade executable paths."},
		},
		Flags: []FlagHelp{
			{Name: "--vsix", Value: "<path>", Description: "VS Code extension package. Defaults to bundled or source-checkout VSIX when available."},
			{Name: "--editor", Value: "<name>", Description: "Editor command: code, cursor, or windsurf."},
			{Name: "--force", Description: "Force extension installation."},
			{Name: "--json", Description: "Write editor doctor status as JSON."},
		},
		Examples: []string{"glade editor doctor vscode", "glade editor install vscode --force"},
	},
	{
		Name:        "dap",
		Description: "Run the Debug Adapter Protocol server over stdio.",
		Usage:       []string{"glade dap [--project <root>] [--db <path>] [--dry-run]"},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Default project root for launch requests."},
			{Name: "--db", Value: "<path>", Description: "SQLite local org path for DB-backed debug sessions."},
			{Name: "--dry-run", Description: "Debug against the selected local org without saving changes."},
		},
		Examples: []string{"glade dap --project .", "glade dap --project . --db .glade/envs/dev.sqlite"},
	},
	{
		Name:        "test",
		Description: "Discover and run supported Apex tests.",
		Usage:       []string{"glade test [--project <root>] [flags]", "glade test changed [--project <root>] [--since <ref>]", "glade test failed [--project <root>]", "glade test serve [--project <root>] [serve flags]", "glade test daemon status|stop [--project <root>]", "glade test clear-cache [--project <root>]"},
		Subcommands: []SubcommandHelp{
			{Name: "changed", Description: "Run tests affected since a git ref. Defaults to HEAD."},
			{Name: "failed", Description: "Rerun tests that failed in the last run."},
			{Name: "serve", Description: "Keep the local test runtime warm over a socket."},
			{Name: "daemon", Description: "Show or stop the persistent test server."},
			{Name: "clear-cache", Description: "Remove the startup cache for a project."},
		},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--filter", Value: "<pattern>", Description: "Run matching test classes or methods."},
			{Name: "--class", Value: "<name>", Description: "Run one exact test class."},
			{Name: "--method", Value: "<name>", Description: "Run one exact test method. Requires --class."},
			{Name: "--class-file", Value: "<path>", Description: "Read exact test class names, one per line."},
			{Name: "--connect", Description: "Require a running test server."},
			{Name: "--no-serve", Description: "Do not auto-connect to a running test server."},
			{Name: "--no-cache", Description: "Skip the on-disk startup cache."},
			{Name: "--last-failed", Description: "Rerun tests that failed in the last completed run."},
			{Name: "--wizard", Description: "Print daily test loop command suggestions."},
			{Name: "--daemon", Description: "Keep index warm in process for watch loops."},
			{Name: "--json", Description: "Write JSON test results."},
			{Name: "--junit", Value: "<path>", Description: "Write JUnit XML results."},
			{Name: "--trace", Value: "<path>", Description: "Write a Chrome trace JSON document for this run."},
			{Name: "--services", Value: "<path>", Description: "Validate a services.yml virtualization config."},
			{Name: "--progress", Description: "Print line progress to stderr."},
			{Name: "--progress-json", Description: "Print NDJSON progress events to stderr."},
			{Name: "--no-progress", Description: "Disable terminal progress."},
			{Name: "--quiet", Description: "Alias for --no-progress."},
			{Name: "--debug", Description: "Run one DAP snapshot session after tests."},
			{Name: "--watch", Description: "Watch source files and emit NDJSON events."},
			{Name: "--watch-once", Description: "Run one watch cycle and exit."},
			{Name: "--changed-since", Value: "<ref>", Description: "Select tests affected since a git ref."},
			{Name: "--since", Value: "<ref>", Description: "Git ref for glade test changed. Defaults to HEAD."},
			{Name: "--debounce", Value: "<dur>", Description: "Watch debounce interval."},
			{Name: "--watch-backend", Value: "<mode>", Description: "Watch backend: auto, native, or poll."},
			{Name: "--parallel-methods", Description: "Run test methods in parallel."},
			{Name: "--no-parallel-methods", Description: "Run methods serially within a class."},
			{Name: "--parallelism", Value: "<n>", Description: "Worker count."},
			{Name: "--test-timeout", Value: "<dur>", Description: "Per-test timeout."},
			{Name: "--gc-aggressive", Description: "Run with GOGC=50."},
			{Name: "--limit-mode", Value: "<mode>", Description: "Governor limit mode: permissive or strict."},
		},
		Examples: []string{"glade test serve --project .", "glade test --project . --class AccountServiceTest", "glade test --project . --class AccountServiceTest --method testCreatesAccount", "glade test --project . --class-file tests.txt"},
	},
	{
		Name:        "dev",
		Description: "Run the human-focused local development cockpit.",
		Usage:       []string{"glade dev [--project <root>]", "glade dev test [--project <root>] [--class <name>|--test <Class.method>|--changed|--failed] [--out <runs-dir>]", "glade dev watch [--project <root>] [--out <runs-dir>]", "glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]", "glade dev lwc [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]"},
		Subcommands: []SubcommandHelp{
			{Name: "test", Description: "Run a saved human-friendly test workflow."},
			{Name: "watch", Description: "Watch and save run artifacts."},
			{Name: "vf", Description: "Start a local Visualforce development server."},
			{Name: "lwc", Description: "Start a local LWC development shell."},
		},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--out", Value: "<runs-dir>", Description: "Run artifact directory."},
			{Name: "--all", Description: "Run all tests."},
			{Name: "--class", Value: "<name>", Description: "Run one test class."},
			{Name: "--test", Value: "<Class.method>", Description: "Run one test method."},
			{Name: "--changed", Description: "Run tests affected since HEAD."},
			{Name: "--failed", Description: "Rerun tests from the last failed run."},
			{Name: "--watch", Description: "Watch during dev test."},
			{Name: "--watch-once", Description: "Run one watch cycle and exit."},
		},
		Examples: []string{"glade dev --project .", "glade dev test --project . --failed"},
	},
	{
		Name:        "report",
		Description: "List saved run reports and generate enterprise reports.",
		Usage:       []string{"glade report assess [--project <root>] [--format json|html|md] [--out <path>]", "glade report cruft [--project <root>] [--format json|html|md] [--out <path>]", "glade report refactor-proof [--project <root>] [--since <ref>] [--format json|html|md] [--out <path>]", "glade report list [--runs-dir <path>]", "glade report show latest [--runs-dir <path>] [--json]", "glade report github latest [--runs-dir <path>]", "glade report export latest --output <path> [--format zip|html] [--runs-dir <path>]", "glade report clean [--runs-dir <path>] [--keep <n>]"},
		Subcommands: []SubcommandHelp{
			{Name: "assess", Description: "Generate an enterprise assessment report."},
			{Name: "cruft", Description: "Generate a conservative cruft report."},
			{Name: "refactor-proof", Description: "Generate a local proof report for a branch change."},
			{Name: "list", Description: "List saved run reports."},
			{Name: "show", Description: "Show the latest run report."},
			{Name: "github", Description: "Emit GitHub Actions annotations for the latest run."},
			{Name: "export", Description: "Export the latest report as zip or HTML."},
			{Name: "clean", Description: "Delete old run reports."},
		},
		Flags: []FlagHelp{
			{Name: "--runs-dir", Value: "<path>", Description: "Run artifact directory."},
			{Name: "--project", Value: "<root>", Description: "Project root for enterprise reports."},
			{Name: "--output", Value: "<path>", Description: "Export output path."},
			{Name: "--out", Value: "<path>", Description: "Enterprise report output path."},
			{Name: "--format", Value: "<mode>", Description: "Report format."},
			{Name: "--since", Value: "<ref>", Description: "Git ref for refactor proof."},
			{Name: "--include-metadata", Description: "Include metadata files in assessment scans."},
			{Name: "--include-tests", Description: "Include test classes in assessment pattern scans."},
			{Name: "--strict", Description: "Promote assessment diagnostics to failure findings."},
			{Name: "--trace", Value: "<path>", Description: "Trace JSON to summarize in refactor proof."},
			{Name: "--fail-on-api-break", Description: "Fail refactor proof when public/global API changes."},
			{Name: "--json", Description: "Write latest report metadata and results as JSON."},
			{Name: "--keep", Value: "<n>", Description: "Number of newest reports to keep."},
		},
		Examples: []string{"glade report assess --project . --format html --out reports/glade-assessment.html", "glade report cruft --project . --format json", "glade report refactor-proof --project . --since origin/main --format json", "glade report show latest --json"},
	},
	{
		Name:        "lsp",
		Description: "Run the Language Server Protocol server over stdio.",
		Usage:       []string{"glade lsp [--project <root>] [--diagnostics-once]"},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--diagnostics-once", Description: "Write one diagnostics batch and exit."},
		},
		Examples: []string{"glade lsp --project . --diagnostics-once"},
	},
	{
		Name:        "profile",
		Description: "Analyze Glade native trace output.",
		Usage:       []string{"glade profile analyze <trace.json> [--json]"},
		Subcommands: []SubcommandHelp{{Name: "analyze", Description: "Analyze a Glade native trace JSON file."}},
		Flags: []FlagHelp{
			{Name: "--json", Description: "Write profile report as JSON."},
			{Name: "--format", Value: "<mode>", Description: "Profile output format: text or markdown."},
		},
		Examples: []string{"glade profile analyze trace.json", "glade profile analyze trace.json --json", "glade profile analyze trace.json --format markdown"},
	},
	{
		Name:        "examples",
		Description: "List and inspect bundled playground examples.",
		Usage:       []string{"glade examples [--tag <tag>]", "glade examples show <id>", "glade examples run <id>"},
		Flags:       []FlagHelp{{Name: "--tag", Value: "<tag>", Description: "Filter examples by tag."}},
		Examples:    []string{"glade examples", "glade examples show account-service", "glade examples run account-service"},
	},
	{
		Name:        "explain",
		Description: "Explain Glade diagnostic codes.",
		Usage:       []string{"glade explain <error-code>"},
		Examples:    []string{"glade explain GLADESEMA002"},
	},
	{
		Name:        "support",
		Description: "Print commands and artifacts useful for support reports.",
		Usage:       []string{"glade support"},
		Examples:    []string{"glade support"},
	},
	{
		Name:        "plugins",
		Description: "Find, install, and manage Glade plugins.",
		Usage:       []string{"glade plugins <command> [flags]"},
		Subcommands: []SubcommandHelp{
			{Name: "list", Description: "List installed plugins."},
			{Name: "available", Description: "List plugins available to install."},
			{Name: "search", Description: "Search the plugin marketplace."},
			{Name: "info", Description: "Show marketplace plugin metadata."},
			{Name: "install", Description: "Install a plugin from the marketplace, registry, URL, or archive."},
			{Name: "link", Description: "Link a local plugin executable."},
			{Name: "remove", Description: "Remove an installed plugin."},
			{Name: "which", Description: "Show the plugin that owns a command."},
			{Name: "doctor", Description: "Check installed plugins."},
			{Name: "lock", Description: "Write glade.plugins.lock.json."},
			{Name: "restore", Description: "Restore plugins from glade.plugins.lock.json."},
		},
		Examples: []string{"glade plugins available", "glade plugins install @glade/compat", "glade plugins install @glade/performance", "glade plugins search quality"},
	},
	{
		Name:        "package",
		Description: "Build, inspect, validate, and diff managed package artifacts.",
		Usage: []string{
			"glade package build --project <root> --namespace <namespace> --output <artifact> [--version <version>] [--json] [--progress|--progress-json|--no-progress]",
			"glade package info <artifact.json> [--json]",
			"glade package validate <artifact.json> [--json]",
			"glade package diff <from.json> <to.json> [--json]",
		},
		Subcommands: []SubcommandHelp{
			{Name: "build", Description: "Build a package artifact JSON file."},
			{Name: "info", Description: "Print artifact metadata and counts."},
			{Name: "validate", Description: "Check artifact shape before publishing or installing."},
			{Name: "diff", Description: "Compare two package artifact JSON files."},
		},
		Flags: []FlagHelp{
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--namespace", Value: "<namespace>", Description: "Managed package namespace."},
			{Name: "--output", Value: "<artifact>", Description: "Artifact JSON path."},
			{Name: "--version", Value: "<version>", Description: "Optional package version."},
			{Name: "--json", Description: "Write structured JSON output."},
			{Name: "--progress", Description: "Print line progress to stderr for package build."},
			{Name: "--progress-json", Description: "Print NDJSON progress events to stderr."},
			{Name: "--no-progress", Description: "Disable terminal progress."},
		},
		Examples: []string{
			"glade package build --project . --namespace pkg --output pkg.json --progress",
			"glade package info pkg.json --json",
			"glade package diff old.json new.json",
		},
	},
	{
		Name:        "server",
		Description: "Start the local Salesforce-compatible API baseline.",
		Usage:       []string{"glade server [--addr <host:port>] [--project <root>] [--db <path>] [--limit-mode <mode>]"},
		Flags: []FlagHelp{
			{Name: "--addr", Value: "<host:port>", Description: "Bind address. Defaults to 127.0.0.1:8080."},
			{Name: "--db", Value: "<path>", Description: "Persistent local database."},
			{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
			{Name: "--limit-mode", Value: "<mode>", Description: "Governor limit mode: permissive or strict."},
		},
		Examples: []string{"glade server --project .", "glade server --db .glade/org.sqlite"},
	},
	{
		Name:        "playground",
		Description: "Start the local Apex playground web UI.",
		Usage: []string{
			"glade playground [--addr <host:port>] [--workspace <id>] [--data-root <path>] [--project <root>] [--db <path>] [--open|--no-open]",
			"glade playground --example <id> [--once|--open]",
			"glade playground --list-examples [--project-ref <name=path>]",
			"glade playground --wizard [--project <root>] [--examples]",
		},
		Flags: []FlagHelp{
			{Name: "--addr", Value: "<host:port>", Description: "Bind address. Defaults to 127.0.0.1:1789."},
			{Name: "--db", Value: "<path>", Description: "Persistent local database."},
			{Name: "--no-db", Description: "Use memory-only org state and do not write org.sqlite."},
			{Name: "--project", Value: "<root>", Description: "Attach a project to the workspace."},
			{Name: "--project-ref", Value: "<name=path>", Description: "Add a named project reference."},
			{Name: "--examples", Description: "Show bundled examples."},
			{Name: "--list-examples", Description: "Print example ids, names, file counts, and tags without serving."},
			{Name: "--example", Value: "<id>", Description: "Start on a bundled example in the managed scratch workspace."},
			{Name: "--reset-on-start", Description: "Clear scratch workspace and org state before serving; refuses --project."},
			{Name: "--public", Description: "Bind to PORT on all interfaces."},
			{Name: "--run-timeout", Value: "<dur>", Description: "Anonymous Apex run timeout."},
			{Name: "--rate-per-minute", Value: "<n>", Description: "Request rate limit."},
			{Name: "--data-root", Value: "<path>", Description: "Playground data directory."},
			{Name: "--workspace", Value: "<id>", Description: "Workspace identifier."},
			{Name: "--limit-mode", Value: "<mode>", Description: "Default limit mode."},
			{Name: "--open", Description: "Open the browser."},
			{Name: "--no-open", Description: "Do not open the browser."},
			{Name: "--once", Description: "Prepare and exit without serving."},
			{Name: "--wizard", Description: "Print a ready playground command without serving."},
		},
		Examples: []string{
			"glade playground --project . --open",
			"glade playground --example deal-desk-discount-guard --once",
			"glade playground --list-examples",
			"glade playground --wizard --project . --examples",
		},
	},
	{
		Name:        "db",
		Description: "Seed, reset, export, and inspect a persistent local database.",
		Usage:       []string{"glade db seed --db <path> [--project <root>] [--json] [--progress|--progress-json|--no-progress] <fixture.json>", "glade db seed --wizard --db <path> [--project <root>] <fixture.json>", "glade db reset --db <path> [--project <root>] [--json]", "glade db export --db <path> [--project <root>]", "glade db inspect --db <path> [--project <root>] [--json]"},
		Subcommands: []SubcommandHelp{
			{Name: "seed", Description: "Apply a fixture to a persistent database."},
			{Name: "reset", Description: "Clear data from a persistent database."},
			{Name: "export", Description: "Write a fixture from a database."},
			{Name: "inspect", Description: "Print database counts."},
		},
		Flags: []FlagHelp{
			{Name: "--db", Value: "<path>", Description: "Persistent local database path."},
			{Name: "--project", Value: "<root>", Description: "Project root for schema bootstrap."},
			{Name: "--json", Description: "Write inspect output as JSON."},
			{Name: "--wizard", Description: "Print a seed and inspect command pair."},
			{Name: "--progress", Description: "Print line progress to stderr for seed."},
			{Name: "--progress-json", Description: "Print NDJSON progress events to stderr."},
			{Name: "--no-progress", Description: "Disable terminal progress."},
		},
		Examples: []string{"glade db inspect --db .glade/org.sqlite", "glade db seed --wizard --db .glade/org.sqlite fixture.json", "glade db seed --db .glade/org.sqlite --progress fixture.json"},
	},
	{
		Name:        "completion",
		Description: "Generate shell completion scripts.",
		Usage:       []string{"glade completion bash|zsh|fish"},
		Subcommands: []SubcommandHelp{
			{Name: "bash", Description: "Write a bash completion script."},
			{Name: "zsh", Description: "Write a zsh completion script."},
			{Name: "fish", Description: "Write a fish completion script."},
		},
		Examples: []string{"glade completion zsh > ~/.zsh/completions/_glade", "glade completion fish > ~/.config/fish/completions/glade.fish"},
	},
	{
		Name:        "help",
		Description: "Print this help text.",
		Usage:       []string{"glade help [command]"},
		Examples:    []string{"glade help", "glade help test"},
	},
}

func projectProgressFlags(jsonDescription string) []FlagHelp {
	return []FlagHelp{
		{Name: "--project", Value: "<root>", Description: "Project root. Defaults to current directory."},
		{Name: "--json", Description: jsonDescription},
		{Name: "--progress", Description: "Print line progress to stderr."},
		{Name: "--progress-json", Description: "Print NDJSON progress events to stderr."},
		{Name: "--no-progress", Description: "Disable terminal progress."},
		{Name: "--quiet", Description: "Alias for --no-progress."},
	}
}

func CommandReferences() []CommandHelp {
	out := make([]CommandHelp, len(commandReferences))
	copy(out, commandReferences)
	return out
}

func FindCommandHelp(name string) (CommandHelp, bool) {
	for _, ref := range commandReferences {
		if ref.Name == name {
			return ref, true
		}
	}
	return CommandHelp{}, false
}

func CommandNames() []string {
	names := make([]string, 0, len(commandReferences))
	for _, ref := range commandReferences {
		names = append(names, ref.Name)
	}
	return names
}

func WriteHelp(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "Glade — local Apex runtime"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Usage:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  glade <command> [flags]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Start here:"); err != nil {
		return err
	}
	for _, command := range []string{
		"glade doctor",
		"glade check",
		"glade test changed --since origin/main",
		"glade playground --examples --open",
	} {
		if _, err := fmt.Fprintln(w, "  "+command); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Workflows:"); err != nil {
		return err
	}
	workflows := []struct {
		name string
		desc string
	}{
		{"check", "catch Apex source and metadata issues"},
		{"test", "run local Apex tests"},
		{"exec", "run anonymous Apex against local state"},
		{"debug", "parse, profile, and explain Salesforce debug logs"},
		{"server", "serve a local Salesforce-shaped REST API"},
		{"playground", "open the browser workbench"},
	}
	for _, row := range workflows {
		if _, err := fmt.Fprintf(w, "  %-11s %s\n", row.name, row.desc); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "More:"); err != nil {
		return err
	}
	for _, command := range []string{
		"glade help workflows",
		"glade help commands",
		"glade examples",
		"glade support",
		"glade help exit-codes",
	} {
		if _, err := fmt.Fprintln(w, "  "+command); err != nil {
			return err
		}
	}
	return nil
}

func WriteCommandsHelp(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "Glade commands"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	maxName := 0
	for _, ref := range commandReferences {
		if len(ref.Name) > maxName {
			maxName = len(ref.Name)
		}
	}
	for _, ref := range commandReferences {
		if _, err := fmt.Fprintf(w, "  %-*s  %s\n", maxName, ref.Name, ref.Description); err != nil {
			return err
		}
	}
	return nil
}

func WriteWorkflowsHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Workflows

Local check loop:
  glade doctor
  glade check
  glade test changed --since origin/main

Local execution:
  glade exec --project . "System.debug('local');"
  glade db inspect --db .glade/local-org.sqlite

Debug logs:
  glade exec "System.debug('local');"
  glade debug profile --log .glade/logs/latest.log
  glade debug explain --log .glade/logs/latest.log --project .

Browser workbench:
  glade examples
  glade playground --examples --open
`)
	_, err := fmt.Fprintln(w, body)
	return err
}

func WriteSupportHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Glade support

Diagnostics:
  glade doctor
  glade check --json
  glade test --json --no-progress

Artifacts:
  glade report show latest --json
  glade debug profile --log <log>

Include:
  glade version --json
  sfdx-project.json
  glade.yml
`)
	_, err := fmt.Fprintln(w, body)
	return err
}

func WriteErrorCodeHelp(w io.Writer, code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	explanations := map[string]struct {
		title string
		why   string
		try   []string
	}{
		"GLADESEMA002": {
			title: "unknown type",
			why:   "Glade found an Apex type reference that is not present in local Apex, schema, or platform symbols.",
			try:   []string{"glade schema load --project .", "glade check --project ."},
		},
		"GLADESCHEMA001": {
			title: "metadata schema load failed",
			why:   "Local metadata could not be parsed into the schema Glade uses for semantic checks.",
			try:   []string{"glade schema load --project .", "glade check --project ."},
		},
		"GLADEPARSE000": {
			title: "Apex parse failed",
			why:   "Glade could not parse one of the Apex source files.",
			try:   []string{"glade parse force-app/main/default/classes", "glade check --project ."},
		},
		"GLADECLI001": {
			title: "unknown command",
			why:   "The command name was not part of the Glade CLI surface or an installed plugin.",
			try:   []string{"glade help", "glade help commands", "glade plugins list"},
		},
	}
	info, ok := explanations[code]
	if !ok {
		return fmt.Errorf("unknown error code %q", code)
	}
	if _, err := fmt.Fprintf(w, "%s — %s\n\n", code, info.title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Why:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  "+info.why); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Try:"); err != nil {
		return err
	}
	for _, step := range info.try {
		if _, err := fmt.Fprintln(w, "  "+step); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "Docs:\n  https://glade.sh/guide/errors#%s\n", strings.ToLower(code))
	return err
}

func WriteCommandHelp(w io.Writer, args []string) error {
	if len(args) == 0 {
		return WriteHelp(w)
	}
	ref, ok := FindCommandHelp(args[0])
	if !ok {
		return WriteHelp(w)
	}
	t := NewTheme(w)
	if _, err := fmt.Fprintln(w, t.Bold("glade "+ref.Name)+" — "+ref.Description); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Usage:"); err != nil {
		return err
	}
	for _, usage := range ref.Usage {
		if _, err := fmt.Fprintln(w, "  "+usage); err != nil {
			return err
		}
	}
	if len(ref.Subcommands) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Subcommands:"); err != nil {
			return err
		}
		maxName := 0
		for _, sub := range ref.Subcommands {
			if len(sub.Name) > maxName {
				maxName = len(sub.Name)
			}
		}
		for _, sub := range ref.Subcommands {
			if _, err := fmt.Fprintf(w, "  %-*s  %s\n", maxName, sub.Name, sub.Description); err != nil {
				return err
			}
		}
	}
	if len(ref.Flags) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Flags:"); err != nil {
			return err
		}
		maxFlag := 0
		for _, flag := range ref.Flags {
			if len(flag.Display()) > maxFlag {
				maxFlag = len(flag.Display())
			}
		}
		for _, flag := range ref.Flags {
			if _, err := fmt.Fprintf(w, "  %-*s  %s\n", maxFlag, flag.Display(), flag.Description); err != nil {
				return err
			}
		}
	}
	if ref.Name == "completion" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Install:"); err != nil {
			return err
		}
		for _, line := range []string{
			"bash: source <(glade completion bash)",
			"zsh:  glade completion zsh > ~/.zsh/completions/_glade",
			"fish: glade completion fish > ~/.config/fish/completions/glade.fish",
		} {
			if _, err := fmt.Fprintln(w, "  "+line); err != nil {
				return err
			}
		}
	}
	if len(ref.Examples) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Examples:"); err != nil {
			return err
		}
		for _, example := range ref.Examples {
			if _, err := fmt.Fprintln(w, "  "+example); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteExitCodesHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Exit codes

  0  Command completed without errors.
  1  Command failed, found diagnostics, or tests failed.
  2  Command was not understood, used invalid flags, or hit a usage error.
  3  Project or config discovery failed.
  4  Unsupported local runtime boundary.
  5  External dependency or toolchain failed.
  70 Internal Glade error.
  130 Interrupted by Ctrl-C.

Notes:
  parse, inspect, and check return 1 when diagnostics include errors.
  test returns 1 when any test fails or errors.
  some legacy usage and flag errors still return 1 during migration.
  unknown commands return 2 before command execution starts.
`)
	_, err := fmt.Fprintln(w, body)
	return err
}

func WriteTestHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Run local Apex tests.

Usage:
  glade test [--project <root>] [flags]
  glade test changed [--project <root>] [--since <ref>]
  glade test failed [--project <root>]
  glade test serve [--project <root>] [serve flags]
  glade test daemon status|stop [--project <root>]
  glade test clear-cache [--project <root>]

Persistent test server:
  glade test serve keeps the project runtime warm across CLI invocations.
  It writes .glade/test/serve.sock and serve.pid under the project root.
  Later glade test runs auto-connect when that socket is reachable.
  Use --no-serve to force a local build, or --connect to require the server.

Daily loop:
  glade test changed --project . --since HEAD runs tests affected by changed files.
  glade test failed --project . reruns tests that failed in the last completed run.
  glade test --project . --last-failed is the flag form of the same rerun.
  glade test --project . --wizard prints the likely next command without running it.

Daemon lifecycle:
  glade test serve --project . starts the persistent warm server.
  glade test daemon status --project . shows stopped, stale, warming, or ready state.
  glade test daemon stop --project . shuts down a live server or removes stale files.

Serve flags:
  --project <root>          Project root. Defaults to current directory.
  --socket <path>           Override the unix socket path.
  --no-watch                Do not watch project files for changes.
  --no-warm                 Skip the initial runtime warm on startup.

Common flags:
  --project <root>          Project root. Defaults to current directory.
  --filter <pattern>        Run matching test classes or methods.
  --connect                 Require a running test server (see serve).
  --no-serve                Do not auto-connect to a running test server.
  --no-cache                Skip the on-disk startup cache for this run.
  --last-failed             Rerun tests that failed in the last completed run.
  --wizard                  Print daily test loop command suggestions.
  --daemon                  Keep index warm in-process for --watch loops.
  --json                    Write JSON test results.
  --junit <path>            Write JUnit XML results.
  --progress                Print line progress to stderr, even when not attached to a terminal.
  --progress-json           Print NDJSON progress events to stderr.
  --no-progress, --quiet    Disable terminal progress.
  --debug                   Run one DAP snapshot session after tests.
  --watch                   Watch source files and emit NDJSON events.
  --watch-once              Run one watch cycle and exit.
  --changed-since <ref>     Select tests affected since a git ref.
  --since <ref>             Git ref for glade test changed (default HEAD).
  --debounce <dur>          Watch debounce interval (default 500ms).
  --watch-backend <mode>    Watch backend: auto, native, or poll.
  --parallel-methods        Run test methods in parallel (default).
  --no-parallel-methods     Force serial method execution within a class.
  --parallelism <n>         Worker count (default: GOMAXPROCS).
  --test-timeout <dur>      Per-test timeout (default 5m, e.g. 30s, 2m).
  --gc-aggressive           Run with GOGC=50 for memory-constrained hosts.
  --limit-mode <mode>       Use strict or permissive governor limits.

Startup cache (.glade/test/startup.gob):
  Written after a cold harness build; loaded when file/config/package fingerprints
  still match. Does not store test results. A stale cache can hide new code —
  use clear-cache after git pull or Glade upgrades, and --no-cache to debug.
  See docs/TEST_STARTUP_CACHE.md for freshness rules and recovery.

Examples:
  glade test serve --project .
  glade test daemon status --project .
  glade test changed --project . --since HEAD
  glade test failed --project .
  glade test clear-cache --project .
  glade test --project . --class AccountServiceTest
  glade test --project . --class AccountServiceTest --method testCreatesAccount
  glade test --project . --no-cache --class AccountServiceTest
  glade test --project . --connect --class AccountServiceTest
  glade test --project . --daemon --watch
  glade test --project . --changed-since origin/main --json --no-progress
`)
	_, err := fmt.Fprintln(w, body)
	return err
}
