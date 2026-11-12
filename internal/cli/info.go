package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diphyx/shipo/internal/check"
	"github.com/diphyx/shipo/internal/config"
	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/scope/publish"
	"github.com/diphyx/shipo/internal/scope/version"
	"github.com/diphyx/shipo/internal/skill"
	"github.com/diphyx/shipo/internal/text/semver"
	"github.com/diphyx/shipo/internal/tool/git"
	"github.com/diphyx/shipo/internal/ui"
)

// ShowVersion prints the target and its version.
func (app *App) ShowVersion(context context.Context) error {
	target, openError := release.Open(context, app.Runner, app.Dir)
	if openError != nil {
		return openError
	}

	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return readError
	}

	source := target.Version.Source
	if target.Version.IsGit {
		source = "git tags (" + target.Version.Patterns[0] + ")"
	}

	rows := [][2]string{
		{"target", folderName(target.Folder)},
		{"preset", target.Version.Preset},
		{"source", source},
		{"version", current.Version.String()},
		{"tag", tagState(context, target, target.Version.TagName(target.Folder, current.Version.String()))},
	}

	for _, row := range rows {
		app.UI.Field(row[0], row[1])
	}

	return nil
}

// Check prints the checks, the state of the current release, and the plan.
func (app *App) Check(context context.Context) error {
	target, openError := release.Open(context, app.Runner, app.Dir)
	if openError != nil {
		return openError
	}

	next, nextError := release.NewRelease(context, target, semver.LEVEL_PATCH)
	if nextError != nil {
		return nextError
	}

	current, headError := release.FromHead(context, target)
	isAtRelease := headError == nil

	options := check.Everything()
	options.IsBump = !isAtRelease

	checks := check.Run(context, next, options)

	app.UI.Heading("checks", true)
	printItems(app.UI, checks)

	if isAtRelease {
		app.UI.Heading("release "+current.Tag, false)
		printItems(app.UI, check.State(context, current))
	} else {
		app.UI.Heading("release", false)
		app.UI.Note("HEAD is not a release; the next one starts with shipo or shipo bump")

		current = next
	}

	app.UI.Heading("plan", false)

	for _, task := range publish.Tasks {
		if !publish.IsSet(target, task) {
			continue
		}

		for index, line := range publish.Describe(current, task) {
			name := ""
			if index == 0 {
				name = task
			}

			app.UI.Row(name, line)
		}
	}

	return check.Error(checks)
}

func printItems(output *ui.Terminal, items []check.Item) {
	for _, item := range items {
		mark := ui.MARK_DONE
		if !item.IsOK {
			mark = ui.MARK_FAIL
		}

		output.Item(mark, item.Name, item.Detail)
	}
}

// Init writes a .shipo for the detected stack in the current folder.
func (app *App) Init() error {
	path := filepath.Join(app.Dir, config.FILE_NAME)

	_, statError := os.Stat(path)
	if statError == nil {
		return fmt.Errorf("%s already exists", config.FILE_NAME)
	}

	preset, detectError := version.Detect(app.Dir)
	if detectError != nil {
		return detectError
	}

	content := fmt.Sprintf(`# shipo release config. See https://github.com/diphyx/shipo

[version]
preset = %q

[bump]
level = "ask"

[publish]
push = "origin/main"
`, preset.Name)

	writeError := os.WriteFile(path, []byte(content), 0o644)
	if writeError != nil {
		return writeError
	}

	app.UI.Done("init", config.FILE_NAME+" ("+preset.Name+")")

	return nil
}

// Skill installs the AI skill for a tool, in the project or for the user.
func (app *App) Skill(arguments []string) error {
	if len(arguments) > 2 {
		return errors.New("usage: shipo skill [tool] [project|user]")
	}

	toolName, scope := "", ""
	if len(arguments) > 0 {
		toolName = arguments[0]
	}

	if len(arguments) > 1 {
		scope = arguments[1]
	}

	if toolName == "" {
		choices := make([]ui.Choice, len(skill.Tools))
		for index, tool := range skill.Tools {
			choices[index] = ui.Choice{Label: tool.Name, Detail: tool.Project}
		}

		index, menuError := app.UI.Menu("Install the shipo skill for:", choices)
		if menuError != nil {
			return fmt.Errorf("%w; usage: shipo skill <%s> <project|user>", menuError, strings.Join(skill.ToolNames(), "|"))
		}

		toolName = skill.Tools[index].Name
	}

	tool, isKnown := skill.LookupTool(toolName)
	if !isKnown {
		return fmt.Errorf("unknown tool %q: want %s", toolName, strings.Join(skill.ToolNames(), ", "))
	}

	if scope == "" {
		choices := []ui.Choice{{Label: skill.SCOPE_PROJECT, Detail: tool.Project}, {Label: skill.SCOPE_USER, Detail: "~/" + tool.User}}

		index, menuError := app.UI.Menu("Install it in:", choices)
		if menuError != nil {
			return fmt.Errorf("%w; usage: shipo skill %s <project|user>", menuError, tool.Name)
		}

		scope = choices[index].Label
	}

	projectRoot := app.Dir

	root, rootError := git.NewGit(app.Runner, app.Dir).Root(context.Background())
	if rootError == nil {
		projectRoot = root
	}

	home, homeError := os.UserHomeDir()
	if homeError != nil {
		return homeError
	}

	path, pathError := skill.Path(tool, scope, projectRoot, home)
	if pathError != nil {
		return pathError
	}

	previous, installError := skill.Install(path, app.Version)
	if installError != nil {
		return installError
	}

	detail := path
	if previous != "" {
		detail += fmt.Sprintf(" (replaced %s with %s)", previous, app.Version)
	}

	app.UI.Done("skill", detail)

	return nil
}

func tagState(context context.Context, target *release.Target, tag string) string {
	if target.Git.HasTag(context, tag) {
		return tag
	}

	return "none yet"
}

func folderName(folder string) string {
	if folder == "." || folder == "" {
		return "."
	}

	return folder + "/"
}
