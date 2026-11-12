// Package cli parses the command line and runs the interactive flow or a
// single step.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/process"
	"github.com/diphyx/shipo/internal/ui"
)

const USAGE = `shipo cuts a release: version → bump → build → publish.

Flow (interactive):
  shipo                              the whole flow, with the bump menu
  shipo major|minor|patch|current    the whole flow, with the bump given

Steps (no prompts):
  shipo bump <level>                 write the version, commit and tag
  shipo build                        run [build]
  shipo push                         push the release commit and tag
  shipo release                      create the GitHub Release
  shipo workflow [--no-wait]         run the workflows
  shipo command                      run the publish commands

Info:
  shipo version                      the target and its version
  shipo check                        checks, release state and plan
  shipo init                         create a .shipo
  shipo skill [tool] [project|user]  install the AI skill

  shipo --version | --help
`

// EXIT_CANCELLED is the exit code after Ctrl-C, as shells use: 128 + SIGINT.
const EXIT_CANCELLED = 130

// App holds what every command needs.
type App struct {
	UI      *ui.Terminal
	Runner  process.Runner
	Dir     string
	Version string
}

// Run runs shipo with the given arguments and returns the exit code.
func Run(arguments []string, version string) int {
	dir, dirError := os.Getwd()
	if dirError != nil {
		fmt.Fprintln(os.Stderr, "error:", dirError)

		return 1
	}

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := &App{UI: ui.NewTerminal(), Runner: process.System{}, Dir: dir, Version: version}

	runError := app.Dispatch(signalContext, arguments)
	if runError != nil && (signalContext.Err() != nil || errors.Is(runError, ui.ErrCancelled)) {
		app.UI.Error(ui.ErrCancelled)

		return EXIT_CANCELLED
	}

	if runError != nil {
		app.UI.Error(runError)

		return 1
	}

	return 0
}

// Dispatch runs one command.
func (app *App) Dispatch(context context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return app.Flow(context, "")
	}

	name, rest := arguments[0], arguments[1:]

	switch name {
	case "--version", "-v":
		{
			app.UI.Print("%s\n", app.Version)

			return nil
		}
	case "--help", "-h", "help":
		{
			app.UI.Print("%s", USAGE)

			return nil
		}
	case string(semver.LEVEL_MAJOR), string(semver.LEVEL_MINOR), string(semver.LEVEL_PATCH), string(semver.LEVEL_CURRENT):
		{
			return app.noExtra(rest, func() error { return app.Flow(context, semver.Level(name)) })
		}
	case "bump":
		{
			if len(rest) != 1 {
				return errors.New("usage: shipo bump <major|minor|patch|current>")
			}

			return app.StepBump(context, rest[0])
		}
	case "build", "push", "release", "command":
		{
			return app.noExtra(rest, func() error { return app.Step(context, name, false) })
		}
	case "workflow":
		{
			isNoWait := len(rest) == 1 && rest[0] == "--no-wait"
			if len(rest) > 0 && !isNoWait {
				return errors.New("usage: shipo workflow [--no-wait]")
			}

			return app.Step(context, name, isNoWait)
		}
	case "version":
		{
			return app.noExtra(rest, func() error { return app.ShowVersion(context) })
		}
	case "check":
		{
			return app.noExtra(rest, func() error { return app.Check(context) })
		}
	case "init":
		{
			return app.noExtra(rest, func() error { return app.Init() })
		}
	case "skill":
		{
			return app.Skill(context, rest)
		}
	}

	return fmt.Errorf("unknown command %q; run shipo --help", name)
}

func (app *App) noExtra(rest []string, run func() error) error {
	if len(rest) > 0 {
		return fmt.Errorf("unexpected argument %q; run shipo --help", rest[0])
	}

	return run()
}
