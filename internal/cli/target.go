package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diphyx/shipo/internal/release"
	"github.com/diphyx/shipo/internal/ui"
)

// LABEL_DEPTH is the deepest folder the target menu shows in full; deeper
// ones show as first/../last.
const LABEL_DEPTH = 3

// takeTarget removes -t <name>, --target <name> or --target=<name> from the
// arguments and returns the name.
func takeTarget(arguments []string) ([]string, string, error) {
	var rest []string

	target := ""

	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]

		switch {
		case argument == "-t" || argument == "--target":
			{
				if index+1 == len(arguments) || strings.HasPrefix(arguments[index+1], "-") {
					return nil, "", fmt.Errorf("usage: shipo %s <target>", argument)
				}

				index++
				target = arguments[index]
			}
		case strings.HasPrefix(argument, "--target="):
			{
				target = strings.TrimPrefix(argument, "--target=")
				if target == "" {
					return nil, "", errors.New("usage: shipo --target=<target>")
				}
			}
		default:
			{
				rest = append(rest, argument)
			}
		}
	}

	return rest, target, nil
}

// openTarget chooses the .shipo to release and opens it. isMenu lets the
// flow, check and version ask; steps never do. isPicked is true when the
// target was picked out of several, by -t or the menu.
func (app *App) openTarget(context context.Context, isMenu bool) (*release.Target, bool, error) {
	isPicked, chooseError := app.chooseTarget(context, isMenu)
	if chooseError != nil {
		return nil, false, chooseError
	}

	target, openError := release.Open(context, app.Runner, app.Dir)

	return target, isPicked, openError
}

// chooseTarget points app.Dir at a target: the -t one, the only one, the
// menu's choice, or the nearest one. Outside a repository, app.Dir stays.
func (app *App) chooseTarget(context context.Context, isMenu bool) (bool, error) {
	targets, discoverError := release.NewTargets(context, app.Runner, app.Dir)
	if discoverError != nil {
		if app.Target != "" {
			return false, discoverError
		}

		return false, nil
	}

	isMany := len(targets.Folders) > 1

	folder, isAsked, pickError := app.pickTarget(context, targets, isMenu)
	if pickError != nil || folder == "" {
		return false, pickError
	}

	app.Dir = targets.Dir(folder)

	return isMany && (app.Target != "" || isAsked), nil
}

// pickTarget returns the folder to use, or "" when there is no target, and
// whether the menu asked for it.
func (app *App) pickTarget(context context.Context, targets *release.Targets, isMenu bool) (string, bool, error) {
	if app.Target != "" {
		folder, matchError := targets.Match(app.Target)

		return folder, false, matchError
	}

	switch len(targets.Folders) {
	case 0:
		{
			return "", false, nil
		}
	case 1:
		{
			return targets.Folders[0], false, nil
		}
	}

	nearest := targets.Nearest(app.Dir)

	if isMenu && app.UI.IsInteractive {
		folder, menuError := app.targetMenu(context, targets, nearest)

		return folder, true, menuError
	}

	if nearest == "" {
		return "", false, fmt.Errorf("no .shipo in this folder or above; pick a target with -t: %s", targets)
	}

	return nearest, false, nil
}

// targetMenu lists the targets with their versions, the nearest one first
// under the cursor.
func (app *App) targetMenu(context context.Context, targets *release.Targets, nearest string) (string, error) {
	selected := 0
	labels := targetLabels(targets.Folders)
	choices := make([]ui.Choice, len(targets.Folders))

	for index, folder := range targets.Folders {
		if folder == nearest {
			selected = index
		}

		choices[index] = ui.Choice{Label: labels[index], Detail: app.targetVersion(context, targets.Dir(folder))}
	}

	index, menuError := app.UI.MenuFrom(context, "target", choices, selected)
	if menuError != nil {
		return "", menuError
	}

	return targets.Folders[index], nil
}

// targetVersion is the current version of the target in dir, for the menu.
func (app *App) targetVersion(context context.Context, dir string) string {
	target, openError := release.Open(context, app.Runner, dir)
	if openError != nil {
		return "invalid .shipo"
	}

	current, readError := target.ReadCurrent(context)
	if readError != nil {
		return "no version"
	}

	return current.Version.String()
}

// targetLabels shortens folders deeper than LABEL_DEPTH to first/../last.
// Folders whose short labels would be the same keep their full path.
func targetLabels(folders []string) []string {
	labels := make([]string, len(folders))
	count := map[string]int{}

	for index, folder := range folders {
		labels[index] = shortFolder(folder)
		count[labels[index]]++
	}

	for index, folder := range folders {
		if count[labels[index]] > 1 {
			labels[index] = folder
		}
	}

	return labels
}

func shortFolder(folder string) string {
	parts := strings.Split(folder, "/")
	if len(parts) <= LABEL_DEPTH {
		return folder
	}

	return parts[0] + "/../" + parts[len(parts)-1]
}
