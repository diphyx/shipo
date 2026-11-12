// Package build runs the [build] commands after bump and before publish.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/text/keyword"
	"github.com/diphyx/shipo/internal/tool/process"
	"github.com/diphyx/shipo/internal/ui"
)

// IsSet reports whether the target has a build.
func IsSet(target *release.Target) bool {
	return len(target.Config.Build.Command) > 0
}

// Dir returns the folder the build runs in.
func Dir(target *release.Target) string {
	return filepath.Join(target.Config.Dir, filepath.FromSlash(target.Config.Build.Dir))
}

// Commands returns the build commands with keywords expanded.
func Commands(plan *release.Release) ([]process.Command, error) {
	values := plan.Values()
	dir := Dir(plan.Target)
	environment := keyword.ExpandAll(plan.Config.Build.Environment, values)

	commands := make([]process.Command, 0, len(plan.Config.Build.Command))

	for _, line := range plan.Config.Build.Command {
		command, parseError := process.Parse(keyword.Expand(line, values))
		if parseError != nil {
			return nil, fmt.Errorf("build.command %q: %w", line, parseError)
		}

		command.Dir = dir
		command.Environment = environment
		commands = append(commands, command)
	}

	return commands, nil
}

// Check validates the build before any change: the folder and the programs.
func Check(target *release.Target) error {
	if !IsSet(target) {
		return nil
	}

	info, statError := os.Stat(Dir(target))
	if statError != nil || !info.IsDir() {
		return fmt.Errorf("build.dir %q does not exist", target.Config.Build.Dir)
	}

	for _, line := range target.Config.Build.Command {
		command, parseError := process.Parse(line)
		if parseError != nil {
			return fmt.Errorf("build.command %q: %w", line, parseError)
		}

		if !isRunnable(target, command.Name) {
			return fmt.Errorf("build.command: %s is not installed", command.Name)
		}
	}

	return nil
}

func isRunnable(target *release.Target, name string) bool {
	if strings.ContainsAny(name, `/\`) {
		return true
	}

	return target.Runner.Look(name)
}

// Run runs every command in order. The first failure stops the rest. The
// build must not change tracked files.
func Run(context context.Context, plan *release.Release, output *ui.Terminal) error {
	commands, commandsError := Commands(plan)
	if commandsError != nil {
		return commandsError
	}

	for index, command := range commands {
		label := fmt.Sprintf("[%d/%d] %s", index+1, len(commands), command.String())
		output.Running("build", label)

		runError := plan.Runner.Run(context, command)
		if runError != nil {
			return runError
		}

		output.Done("build", label)
	}

	changed, changedError := plan.Git.ChangedTracked(context)
	if changedError != nil {
		return changedError
	}

	if len(changed) > 0 {
		return fmt.Errorf("build changed tracked files: %s", strings.Join(changed, ", "))
	}

	return nil
}
