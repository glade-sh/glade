# Copilot Instructions

Follow `AGENTS.md` at the repository root. It is the source of truth for AI and
agent development guidance in this repo.

Keep this repository focused on the framework and tool that users install.
Development scanners, compatibility harnesses, ledgers, and gap-finding
commands live in the sibling `~/Dev/glade-tools` project.

For runtime/API work, use the shared runtime stack already in the repo:
`storage`, `dml`, `soql`, `vm`, `apextest`, `server`, and `testreport`. Add
focused Go tests before changing runtime behavior.

Do not stage or commit the built `glade` binary unless explicitly requested.
