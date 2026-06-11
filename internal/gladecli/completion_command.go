package gladecli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/cliui"
)

func runCompletion(args []string, w io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade completion bash|zsh|fish")
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "bash":
		return writeBashCompletion(w)
	case "zsh":
		return writeZshCompletion(w)
	case "fish":
		return writeFishCompletion(w)
	default:
		return fmt.Errorf("unsupported completion shell %q", args[0])
	}
}

func writeBashCompletion(w io.Writer) error {
	refs := cliui.CommandReferences()
	commands := strings.Join(cliui.CommandNames(), " ")
	if _, err := fmt.Fprintln(w, `# bash completion for glade
_glade_completion()
{
  local cur cmd opts subcommands
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"

  if [[ ${COMP_CWORD} -eq 1 ]]; then`); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "    COMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", commands); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, `    return 0
  fi

  cmd="${COMP_WORDS[1]}"
  opts="--help -h"
  subcommands=""`); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  case \"$cmd\" in"); err != nil {
		return err
	}
	for _, ref := range refs {
		flags := bashFlags(ref)
		subs := subcommandNames(ref)
		if _, err := fmt.Fprintf(w, "    %s)\n", ref.Name); err != nil {
			return err
		}
		if len(subs) > 0 {
			if _, err := fmt.Fprintf(w, "      # command: %s %s\n", ref.Name, strings.Join(subs, " ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "      opts=%q\n", strings.Join(flags, " ")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "      subcommands=%q\n", strings.Join(subs, " ")); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "      ;;"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, `  esac

  if [[ "$cur" == -* ]]; then
    COMPREPLY=( $(compgen -W "$opts" -- "$cur") )
    return 0
  fi
  if [[ ${COMP_CWORD} -eq 2 && -n "$subcommands" ]]; then
    COMPREPLY=( $(compgen -W "$subcommands" -- "$cur") )
    return 0
  fi
}

complete -F _glade_completion glade`); err != nil {
		return err
	}
	return nil
}

func writeZshCompletion(w io.Writer) error {
	refs := cliui.CommandReferences()
	if _, err := fmt.Fprintln(w, `#compdef glade

_glade() {
  local -a commands
  commands=(`); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := fmt.Fprintf(w, "    %q\n", ref.Name+":"+strings.TrimSuffix(ref.Description, ".")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, `  )

  if (( CURRENT == 2 )); then
    _describe -t commands 'glade commands' commands
    return
  fi

  case ${words[2]} in`); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := fmt.Fprintf(w, "    %s)\n", ref.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "      _arguments -C \\"); err != nil {
			return err
		}
		if subs := subcommandNames(ref); len(subs) > 0 {
			if _, err := fmt.Fprintf(w, "        '1:subcommand:(%s)' \\\n", strings.Join(subs, " ")); err != nil {
				return err
			}
		}
		for _, flag := range zshFlags(ref) {
			if _, err := fmt.Fprintf(w, "        %q \\\n", flag); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "        '*:file:_files'"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "      ;;"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, `  esac
}

_glade "$@"`); err != nil {
		return err
	}
	return nil
}

func writeFishCompletion(w io.Writer) error {
	refs := cliui.CommandReferences()
	if _, err := fmt.Fprintln(w, "# fish completion for glade"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "complete -c glade -f"); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := fmt.Fprintf(w, "complete -c glade -n '__fish_use_subcommand' -a %s -d %s\n", fishQuote(ref.Name), fishQuote(strings.TrimSuffix(ref.Description, "."))); err != nil {
			return err
		}
	}
	for _, ref := range refs {
		subs := subcommandNames(ref)
		if len(subs) > 0 {
			if _, err := fmt.Fprintf(w, "complete -c glade -n '__fish_seen_subcommand_from %s' -a %s\n", ref.Name, fishQuote(strings.Join(subs, " "))); err != nil {
				return err
			}
		}
		for _, flag := range ref.Flags {
			name := strings.TrimPrefix(flag.Name, "--")
			requiresValue := ""
			if flag.Value != "" {
				requiresValue = " -r"
			}
			if _, err := fmt.Fprintf(w, "complete -c glade -n '__fish_seen_subcommand_from %s' -l %s%s -d %q\n", ref.Name, name, requiresValue, flag.Description); err != nil {
				return err
			}
		}
	}
	return nil
}

func fishQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `\'`) + "'"
}

func bashFlags(ref cliui.CommandHelp) []string {
	flags := []string{"--help", "-h"}
	for _, flag := range ref.Flags {
		flags = append(flags, flag.Name)
	}
	return flags
}

func zshFlags(ref cliui.CommandHelp) []string {
	flags := []string{"--help[Show command help]", "-h[Show command help]"}
	for _, flag := range ref.Flags {
		desc := strings.TrimSuffix(flag.Description, ".")
		if flag.Value != "" {
			name := strings.TrimPrefix(flag.Name, "--")
			flags = append(flags, fmt.Sprintf("%s[%s]:%s:_files", flag.Name, desc, strings.Trim(flag.Value, "<>")))
			if name == "project" || name == "db" || name == "output" || name == "trace" || name == "debug-log" || name == "runs-dir" || name == "data-root" || name == "vsix" {
				flags[len(flags)-1] = fmt.Sprintf("%s[%s]:%s:_files", flag.Name, desc, strings.Trim(flag.Value, "<>"))
			}
			continue
		}
		flags = append(flags, fmt.Sprintf("%s[%s]", flag.Name, desc))
	}
	return flags
}

func subcommandNames(ref cliui.CommandHelp) []string {
	names := make([]string, 0, len(ref.Subcommands))
	for _, sub := range ref.Subcommands {
		names = append(names, sub.Name)
	}
	return names
}
