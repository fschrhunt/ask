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

const docsURL = "github.com/fschrhunt/ask/tree/main/docs"

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
		return "  None; see ask help agents"
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
  batch      Run many tasks in parallel
  show       Print a saved answer
  runs       List recent runs
  stop       Stop a running run
  models     List available models
  title      Name a command for a host
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
  --json, --schema F   Require JSON, or JSON matching a schema
  --no-hooks           Skip your hooks

Models
`+modelLines(a), "ask help run for run options · ask COMMAND --help for the rest\ndocs  "+docsURL)
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
  --json               Require a JSON answer
  --schema FILE        Require JSON matching this schema
  --no-hooks           Skip your hooks

Output
  stdout   The answer, and nothing else
  stderr   Status lines: ask RUN · state · model · details
  exit     0 ok · 1 failed · 2 usage error

Examples
  ask -m claude:haiku-4.5 "Where is login checked?"
  ask -c k3f9a2 -w "Fix it and run the test."` + "\n\ndocs  " + docsURL + "/usage.md"
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
  json       Require a JSON answer
  schema     A JSON Schema, inline

Results · a JSON array on stdout, in task order
  run, id           Run and task ids
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
  -j N                         Tasks at once (default: 4)
  --no-hooks                   Skip your hooks

Examples
  ask batch -j 4 -m claude:haiku-4.5 tasks.json
  ask batch --resume p81c0d` + "\n\ndocs  " + docsURL + "/batches.md"
	case "show":
		return `ask show · print a run again, without starting its agent

Usage
  ask show RUN [--json]
  ask show RUN/TASK [--json]

Output
  stdout   The answer; with --json, the whole result
  stderr   The run's status line

Examples
  ask show k3f9a2
  ask show p81c0d/api --json` + "\n\ndocs  " + docsURL + "/runs.md"
	case "runs":
		return `ask runs · list recent runs, newest first

Usage
  ask runs [-n N]

Options
  -n N   How many (default: 20)

Output
  RUN  STARTED  STATUS  MODEL  TIME  TASK

Examples
  ask runs
  ask runs -n 50` + "\n\ndocs  " + docsURL + "/runs.md"
	case "stop":
		return `ask stop · stop a run and its agents

Usage
  ask stop RUN

Output
  stderr   Stopped, or not running
  exit     0 stopped · 1 not running

Example
  ask stop k3f9a2` + "\n\ndocs  " + docsURL + "/runs.md"
	case "models":
		return `ask models · list the models you can use

Usage
  ask models [--names]

Output
  piped      One agent:id per line
  terminal   Grouped by agent, with each model's name
  --names    agent:id and name, tab-separated, everywhere

Examples
  ask models
  ask models --names` + "\n\ndocs  " + docsURL + "/models.md"
	case "title":
		return `ask title · name a command for a host's task list

Usage
  ask title --command STRING [--description TEXT]

Output
  stdout   MODEL · JOB, plus · write or · worktree when it applies;
           nothing when the command runs no task
  exit     0, or 2 when ask title itself is misused

Hooks
  title hooks may rename it; --no-hooks in the command skips them

Example
  ask title --command 'ask -m claude:haiku-4.5 "Check tests"' --description "check tests"` + "\n\ndocs  " + docsURL + "/hosts.md"
	case "install":
		return `ask install · install a package, or update every package

Usage
  ask install [SOURCE]

Source
  OWNER/REPO   A GitHub repository
  URL          Any git repository
  PATH         A local git repository

Examples
  ask install owner/repo
  ask install` + "\n\ndocs  " + docsURL + "/packages.md"
	case "packages":
		return `ask packages · list installed packages and what they offer

Usage
  ask packages

Example
  ask packages` + "\n\ndocs  " + docsURL + "/packages.md"
	case "remove":
		return `ask remove · remove a package

Usage
  ask remove PACKAGE

Example
  ask remove owner/repo` + "\n\ndocs  " + docsURL + "/packages.md"
	case "hooks":
		return `ask hooks · change tasks before they run and check results after

Hooks are executables in ~/.ask/hooks. The docs have the contract and examples.` + "\n\ndocs  " + docsURL + "/hooks.md"
	case "agents":
		return `ask agents · run a coding agent's CLI for ask

Agents are executables in ~/.ask/agents. The docs have the contract and examples.` + "\n\ndocs  " + docsURL + "/agents.md"
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
