package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fschrhunt/ask/internal/agent"
	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/status"
)

// modelLines groups each local agent's model ids, or why it could not list them, one agent per
// row with the names aligned, wrapped to 80 columns so help reads the same on any terminal or pipe.
func modelLines(a *agent.Registry) string {
	config, err := a.Paths.ReadModels()
	if err != nil {
		return "  " + strings.Join(strings.Fields(err.Error()), " ")
	}
	ids, failures := a.List(config)
	agents := find.Sorted(a.Paths, "agents")
	if len(agents) == 0 {
		return "  None yet; run ask setup"
	}
	width := 0
	for _, entry := range agents {
		width = max(width, len(entry.Name))
	}
	rows := []string{}
	for _, entry := range agents {
		words := []string{}
		for _, id := range ids {
			if name, model, _ := strings.Cut(id, ":"); name == entry.Name {
				words = append(words, model)
			}
		}
		for _, failure := range failures {
			if reason, ok := strings.CutPrefix(failure, entry.Name+": "); ok {
				words = append([]string{"(could not list: " + strings.Join(strings.Fields(reason), " ") + ")"}, words...)
			}
		}
		if len(words) > 12 {
			words = []string{fmt.Sprintf("%d models; ask models lists them, ask setup %s chooses", len(words), entry.Name)}
		}
		if len(words) == 0 {
			words = []string{"(lists none; give an id)"}
		}
		indent := strings.Repeat(" ", 2+width+3)
		line := fmt.Sprintf("  %-*s   %s", width, entry.Name, words[0])
		for _, w := range words[1:] {
			if len(line)+2+len(w) > 80 {
				rows = append(rows, line)
				line = indent + w
			} else {
				line += "  " + w
			}
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

var commandDescription = regexp.MustCompile(`ask-command:\s*(.+)`)

// overview presents the same concise command map to terminals and pipes.
func overview(a *agent.Registry) string {
	lines := []string{`ask · hand tasks to coding agents

Usage
  ask -m MODEL [options] PROMPT    Run a task
  ask -c RUN [options] PROMPT      Follow up in the same conversation

Commands
  setup      Set ask up the first time
  settings   Change agents, defaults, worktrees and apps
  batch      Run many tasks in parallel
  show       Print a saved answer
  wait       Wait for runs to finish, then print them
  runs       List recent runs
  stop       Stop a running run
  clean      Remove finished worktrees and old runs
  models     List available models
  title      Name a command for a host
  docs       Read the docs
  install    Install or update packages
  packages   List installed packages
  remove     Remove a package`}
	commands := find.Sorted(a.Paths, "commands")
	if len(commands) > 0 {
		own := []string{"Your commands"}
		for _, c := range commands {
			desc := ""
			if b, err := os.ReadFile(c.Path); err == nil {
				text := string(b)
				if len(text) > 4096 {
					text = text[:4096]
				}
				if m := commandDescription.FindStringSubmatch(text); m != nil {
					desc = home.Trim(m[1])
				}
			}
			if _, builtin := options[c.Name]; builtin {
				desc = "(hidden: ask's own " + c.Name + " wins; rename yours)"
			}
			own = append(own, fmt.Sprintf("  %-10s %s", c.Name, desc))
		}
		lines = append(lines, strings.Join(own, "\n"))
	}
	lines = append(lines, `Options
  -m, --model MODEL    agent:id[#effort]
  -w, --write          Allow edits and commands (default: read only)
  --worktree           Work in a new git worktree (with -w)
  -C, --dir DIR        Directory to work in
  -t, --timeout S      Seconds per task (default: 900)
  --max-cost USD       Stop a task that spends more (default: none, or your settings)
  --json, --schema F   Require JSON, or JSON matching a schema
  --no-hooks           Skip your hooks

Models
`+modelLines(a), "ask help run for run options · ask COMMAND --help for the rest\ndocs  ask docs")
	return strings.Join(lines, "\n\n")
}

// topicHelp returns one command's page: what it does, usage, its fields or options, what it prints,
// examples, and its docs page. It returns "" for an unknown topic.
func topicHelp(topic string) string {
	switch topic {
	case "run":
		return `ask · run one task, or follow up on one

Usage
  ask -m MODEL [options] PROMPT
  ask -c RUN [options] PROMPT

Options
  -m, --model MODEL    agent:id[#effort], from ask models
  -c, --continue RUN   Follow up on RUN or RUN/TASK, in its conversation
  -w, --write          Allow edits and commands (default: read only)
  --worktree           Work in a new git worktree and branch (with -w)
  -C, --dir DIR        Directory to work in (default: current)
  -t, --timeout S      Seconds per task (default: 900)
  --max-cost USD       Stop a task that spends more (default: none, or your settings)
  --json               Require a JSON answer
  --schema FILE        Require JSON matching this schema
  --no-hooks           Skip your hooks

Output
  stdout   The answer, and nothing else
  stderr   Status lines: ask RUN · state · model · details
  exit     0 ok · 1 failed · 2 usage error

Runs
  RUN is named from the prompt, like login-checked; a follow-up
  keeps the name. The run's id, like k3f9a2, works too.

Examples
  ask -m claude:haiku-4.5 "Where is login checked?"
  ask -c login-checked -w "Fix it and run the test."` + "\n\ndocs  ask docs usage"
	case "batch":
		return `ask batch · run tasks in parallel

Usage
  ask batch [options] FILE|-
  ask batch --resume RUN [-j N] [--no-hooks]

Tasks · a JSON array, or one object per line
  prompt     The task (required)
  id         Result id (default: position)
  model      agent:id[#effort] (default: -m)
  continue   RUN or RUN/TASK to follow up
  write      Allow edits (default: -w)
  worktree   Work in a new git worktree (default: --worktree)
  dir        Directory (default: -C)
  timeout    Seconds (default: -t, else 900)
  max_cost   Dollars this task may spend (default: --max-cost, else your settings)
  json       Require a JSON answer
  schema     A JSON Schema, inline

Results · a JSON array on stdout, in task order
  run, id           RUN/TASK, and the task id
  model, name       Requested model and the model's own name
  ok                true or false
  answer, error     The answer, or why it failed
  seconds, usage    Time, and tokens and cost when reported
  session, dir      Agent session and the directory it worked in
  changes, commits  Files and commits a write run made
  worktree          The kept worktree and branch
  followups         Follow-ups your hooks asked for

Options
  -m, -w, --worktree, -C, -t   Defaults for every task
  --max-cost USD               Each task's cost limit
  -j N                         Tasks at once (default: 4)
  --no-hooks                   Skip your hooks

Examples
  ask batch -j 4 -m claude:haiku-4.5 tasks.json
  ask batch --resume summarize-public-api` + "\n\ndocs  ask docs batches"
	case "show":
		return `ask show · print a run again, without starting its agent

Usage
  ask show RUN [--json]
  ask show RUN/TASK [--json]

Output
  stdout   The answer; with --json, the whole result
  stderr   The run's status line

Examples
  ask show login-checked
  ask show summarize-public-api/api --json` + "\n\ndocs  ask docs runs"
	case "runs":
		return `ask runs · list recent runs, newest first

Usage
  ask runs [-n N] [--all]

Options
  -n N    How many (default: 20)
  --all   Every run, not only this repository's

Output
  RUN  ID  STARTED  STATUS  MODEL  TIME  TASK

Examples
  ask runs
  ask runs --all -n 50` + "\n\ndocs  ask docs runs"
	case "wait":
		return `ask wait · wait for runs to finish, then print them

Usage
  ask wait RUN [RUN...] [-t S] [--json]

Options
  -t S     Give up after S seconds (exit 1)
  --json   One run's whole result, as with ask show

Output
  one run     Its answer on stdout, its status line on stderr, like ask show
  several     Their results as one JSON array on stdout
  exit        0 all ok · 1 one failed, stopped or still running · 2 usage error

Examples
  ask wait login-test-fail
  ask wait fix-api fix-tests -t 1800` + "\n\ndocs  ask docs runs"
	case "clean":
		return `ask clean · remove worktrees whose work has landed, and old runs

Usage
  ask clean [--days N] [--dry-run] [--yes]

Removes
  worktrees   Left by write runs, once clean and their changes are on main
              (merged or squash-merged), or when they changed nothing
  runs        Records older than N days (default 30) without a kept worktree

Options
  --days N    Age of runs to remove (default: 30)
  --dry-run   Show what would go, change nothing
  --yes       Remove without asking (needed without a terminal)

Examples
  ask clean
  ask clean --dry-run --days 7` + "\n\ndocs  ask docs runs"
	case "stop":
		return `ask stop · stop a run and its agents

Usage
  ask stop RUN

Output
  stderr   Stopped, or not running
  exit     0 stopped · 1 not running

Example
  ask stop login-checked` + "\n\ndocs  ask docs runs"
	case "models":
		return `ask models · list the models you can use, and turn them on or off

Usage
  ask models [--names] [--all]
  ask models MODEL... --enable | --disable | --max-cost USD

Output
  piped      One agent:id per line, the models that are on
  terminal   Grouped by agent, with each model's name and cost limit
  --names    agent:id and name, tab-separated, everywhere
  --all      Every model, with on or off

Changes, saved in ~/.ask/models.json
  --enable, --disable   Turn models on or off; off models are hidden and refused
  --max-cost USD        Their cost limit per task; 0 for your default

Examples
  ask models
  ask models --all
  ask models codex:gpt-5.6-sol opencode:gpt-4o --disable
  ask models claude:opus-5.5 --max-cost 10
  ask settings opencode              Choose in a filterable list` + "\n\ndocs  ask docs models"
	case "title":
		return `ask title · name a command for a host's task list

Usage
  ask title --command STRING [--description TEXT]
  ask title --hook        A PreToolUse hook event on stdin, as Claude Code sends it

Output
  stdout   MODEL · JOB, plus · write or · worktree when it applies;
           nothing when the command runs no task
  --hook   The tool input with the title as its description, in the background
  exit     0, or 2 when ask title itself is misused

Hooks
  title hooks may rename it; --no-hooks in the command skips them

Example
  ask title --command 'ask -m claude:haiku-4.5 "Check tests"' --description "check tests"` + "\n\ndocs  ask docs hosts"
	case "install":
		return `ask install · install agents and packages, or update every package

Usage
  ask install [SOURCE...]

Source
  NAME         An official agent, built into ask: claude, codex, opencode
  OWNER/REPO   A GitHub repository
  URL          Any git repository
  PATH         A local git repository, like ./tools or ~/code/tools

Output
  stderr   What was installed, and whether each agent it brings is ready
  exit     0 ready · 1 an agent is not ready · 2 usage error

Examples
  ask install claude codex
  ask install owner/repo
  ask install` + "\n\ndocs  ask docs packages"
	case "packages":
		return `ask packages · list installed packages and what they offer

Usage
  ask packages

Example
  ask packages` + "\n\ndocs  ask docs packages"
	case "remove":
		return `ask remove · remove a package

Usage
  ask remove PACKAGE

Example
  ask remove owner/repo` + "\n\ndocs  ask docs packages"
	case "hooks":
		return `ask hooks · change tasks before they run and check results after

Hooks are executables in ~/.ask/hooks. The docs have the contract and examples.` + "\n\ndocs  ask docs hooks"
	case "agents":
		return `ask agents · run a coding agent's CLI for ask

ask setup connects the official agents for the CLIs you have, and ask settings NAME changes
one. Your own are executables in
~/.ask/agents; the docs have the contract and examples.` + "\n\ndocs  ask docs agents"
	case "setup":
		return `ask setup · set ask up the first time, or check it

Usage
  ask setup             In a terminal: a walkthrough, then a review before anything is saved;
                        once set up, your settings
  ask setup --yes       The recommended setup without asking: agents for the CLIs found,
                        the ask skill, task titles in Claude Code; shows what it changes
  ask setup --check     Report, and exit 1 while an installed agent can't run

Options
  --json   With --check, the report as JSON

Examples
  ask setup
  ask setup --check --json` + "\n\ndocs  ask docs setup"
	case "docs":
		return `ask docs · read ask's docs, built in

Usage
  ask docs                  Every page, with what it covers
  ask docs PAGE             One page: styled in a terminal, markdown when piped
  ask docs --search TERM    Every line in the docs that mentions TERM

Options
  --raw     Markdown, even in a terminal
  --url     The page on GitHub instead

Pages
  index, install, setup, settings, usage, runs, batches, models, agents, hosts,
  hooks, commands, packages, compatibility, and the official agents: claude,
  codex, opencode. A page's first letters are enough, like ask docs batch.

Examples
  ask docs settings
  ask docs --search worktree
  ask docs claude --raw` + "\n\ndocs  ask docs index"
	case "settings":
		return `ask settings · change agents, defaults, worktrees and apps

Usage
  ask settings                  In a terminal: every setting, grouped; edits wait for review
  ask settings NAME             One agent: install or remove it, its models, their cost limits
  ask settings get [KEY]        Print settings, or one value
  ask settings set KEY VALUE    Show what changes, then save it
  ask settings unset KEY        Back to the default

Keys
  model       The model when -m gives none, like claude:sonnet-5.5
  timeout     Seconds per task (default: 900)
  jobs        Batch tasks at once (default: 4)
  max_cost    Dollars a task may spend; ask stops it there (default: no limit)
  worktrees   Where --worktree works, like ~/code/worktrees/ask-{name}
  branches    A worktree's branch (default: ask/{name})
  hook        on or off: title ask runs in Claude Code's task list
  skills      Apps to give the ask skill: claude-code, codex, opencode, cursor, pi, all

Options
  --dry-run   With set or unset, show what would change and save nothing
  --json      With get, the values that apply, as JSON

Examples
  ask settings
  ask settings opencode
  ask settings set max_cost 2
  ask settings set worktrees '~/code/worktrees/ask-{name}' --dry-run
  ask settings get --json` + "\n\ndocs  ask docs settings"
	}
	return ""
}

// printHelp prints help, styled only on an interactive stdout: the title line's command and every
// section title in bold, the footer dim. Piped, it is the same text without escape codes.
func printHelp(text string) {
	lines := strings.Split(text, "\n")
	if status.CanStyle(os.Stdout) {
		for i, line := range lines {
			switch {
			case i == 0:
				if cmd, rest, ok := strings.Cut(line, " · "); ok {
					lines[i] = "\x1b[1m" + cmd + "\x1b[0m\x1b[2m · " + rest + "\x1b[0m"
				}
			case strings.HasPrefix(line, "docs  ") || strings.HasPrefix(line, "ask help run for"):
				lines[i] = "\x1b[2m" + line + "\x1b[0m"
			case line != "" && line[0] != ' ':
				lines[i] = "\x1b[1m" + line + "\x1b[0m"
			}
		}
	}
	fmt.Fprintln(os.Stdout, strings.Join(lines, "\n"))
}
