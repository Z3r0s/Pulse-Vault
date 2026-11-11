package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type browserList struct {
	widget.List
	open, up func()
}

func (l *browserList) TypedKey(event *fyne.KeyEvent) {
	if event.Name == fyne.KeyReturn || event.Name == fyne.KeyEnter {
		l.open()
		return
	}
	l.List.TypedKey(event)
}

func (l *browserList) TypedShortcut(shortcut fyne.Shortcut) {
	if key, ok := shortcut.(*desktop.CustomShortcut); ok && key.KeyName == fyne.KeyUp && key.Modifier == fyne.KeyModifierAlt {
		l.up()
		return
	}
}

func browseLocal(title, initial string, foldersOnly bool, parent fyne.Window, accept func(string)) {
	home, _ := os.UserHomeDir()
	if initial == "" {
		initial = home
	}
	if initial == "" {
		initial = "."
	}
	location := widget.NewEntry()
	location.SetPlaceHolder("Enter a folder path")
	search := widget.NewEntry()
	search.SetPlaceHolder("Filter by name")
	status := widget.NewLabel("")
	status.Truncation = fyne.TextTruncateEllipsis
	var folder, selected string
	var entries, visible []os.DirEntry
	var picker *dialog.CustomDialog
	var navigate func(string)
	var selectedFolder, loading, closed bool
	var request uint64
	type visit struct {
		path, query, selection string
		offset                 float32
	}
	var history []visit
	current := -1
	var load func(string, int)
	back := widget.NewButton("Back", func() {
		if current > 0 {
			load(history[current-1].path, current-1)
		}
	})
	forward := widget.NewButton("Forward", func() {
		if current+1 < len(history) {
			load(history[current+1].path, current+1)
		}
	})
	back.Disable()
	forward.Disable()
	ancestors := widget.NewSelect(nil, nil)
	ancestors.PlaceHolder = "Parent folders"
	ancestors.OnChanged = func(path string) {
		if path != "" {
			ancestors.ClearSelected()
			navigate(path)
		}
	}
	showDotfiles := widget.NewCheck("Show dotfiles", nil)
	vaultOnly := widget.NewCheck("Vault files only", nil)
	open := func() {
		if loading || selected == "" {
			return
		}
		if selectedFolder {
			navigate(selected)
			return
		}
		picker.Hide()
		accept(selected)
	}
	choose := widget.NewButton("Open file", open)
	choose.Importance = widget.HighImportance
	choose.Disable()
	useFolder := widget.NewButton("Use this folder", func() {
		if !loading && folder != "" {
			picker.Hide()
			accept(folder)
		}
	})
	useFolder.Importance = widget.HighImportance
	useFolder.Disable()
	if foldersOnly {
		choose.Importance = widget.MediumImportance
		choose.SetText("Open folder")
	}
	list := &browserList{}
	list.ExtendBaseWidget(list)
	list.Length = func() int { return len(visible) }
	list.CreateItem = func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Truncation = fyne.TextTruncateEllipsis
		return container.NewBorder(nil, nil, widget.NewIcon(theme.FileIcon()), nil, label)
	}
	list.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		entry := visible[id]
		row := obj.(*fyne.Container)
		row.Objects[0].(*widget.Label).SetText(entry.Name())
		icon := theme.FileIcon()
		if entry.IsDir() {
			icon = theme.FolderIcon()
		}
		row.Objects[1].(*widget.Icon).SetResource(icon)
	}
	filter := func(query string) {
		list.UnselectAll()
		selected = ""
		choose.Disable()
		if loading {
			return
		}
		query = strings.ToLower(strings.TrimSpace(query))
		visible = visible[:0]
		for _, entry := range entries {
			if !showDotfiles.Checked && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if vaultOnly.Checked && !entry.IsDir() {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext != ".pulsevault" && ext != ".vault" {
					continue
				}
			}
			if (!foldersOnly || entry.IsDir()) && strings.Contains(strings.ToLower(entry.Name()), query) {
				visible = append(visible, entry)
			}
		}
		list.Refresh()
		list.ScrollToTop()
		status.SetText(fmt.Sprintf("%d items", len(visible)))
		if len(visible) == 0 {
			status.SetText("No matching items")
		}
	}
	navigate = func(path string) { load(path, -1) }
	load = func(path string, target int) {
		if current >= 0 && !loading {
			history[current] = visit{folder, search.Text, selected, list.GetScrollOffset()}
		}
		request++
		id := request
		previous := folder
		loading = true
		choose.Disable()
		useFolder.Disable()
		status.SetText("Opening folder…")
		go func() {
			path, err := filepath.Abs(strings.TrimSpace(path))
			var next []os.DirEntry
			if err == nil {
				next, err = os.ReadDir(path)
			}
			sort.SliceStable(next, func(i, j int) bool {
				return next[i].IsDir() && !next[j].IsDir()
			})
			fyne.Do(func() {
				if closed || id != request {
					return
				}
				loading = false
				if err != nil {
					filter(search.Text)
					if folder != "" {
						useFolder.Enable()
					}
					status.SetText("Cannot open folder: " + err.Error())
					return
				}
				folder, entries = path, next
				location.SetText(folder)
				if target < 0 {
					if current < 0 || history[current].path != folder {
						history = append(history[:current+1], visit{path: folder})
						current++
					}
				} else {
					current = target
				}
				state := history[current]
				if search.Text == state.query {
					filter(state.query)
				} else {
					search.SetText(state.query)
				}
				if target < 0 && previous != folder && filepath.Dir(previous) == folder {
					state.selection = previous
				}
				for index, entry := range visible {
					if filepath.Join(folder, entry.Name()) == state.selection {
						list.Select(index)
						list.ScrollTo(index)
						break
					}
				}
				if target >= 0 {
					list.ScrollToOffset(state.offset)
				}
				back.Disable()
				forward.Disable()
				if current > 0 {
					back.Enable()
				}
				if current+1 < len(history) {
					forward.Enable()
				}
				var parents []string
				for child := folder; filepath.Dir(child) != child; child = filepath.Dir(child) {
					parents = append(parents, filepath.Dir(child))
				}
				ancestors.SetOptions(parents)
				useFolder.Enable()
			})
		}()
	}
	search.OnChanged = filter
	showDotfiles.OnChanged = func(bool) { filter(search.Text) }
	vaultOnly.OnChanged = func(bool) { filter(search.Text) }
	location.OnSubmitted = navigate
	list.OnSelected = func(id widget.ListItemID) {
		entry := visible[id]
		if loading {
			return
		}
		selectedFolder = entry.IsDir()
		selected = filepath.Join(folder, entry.Name())
		if selectedFolder {
			choose.SetText("Open folder")
		} else {
			choose.SetText("Open file")
		}
		status.SetText(entry.Name())
		if !selectedFolder {
			path := selected
			go func() {
				info, err := entry.Info()
				if err == nil {
					fyne.Do(func() {
						if !closed && !loading && selected == path {
							status.SetText(fmt.Sprintf("%s · %d bytes", entry.Name(), info.Size()))
						}
					})
				}
			}()
		}
		choose.Enable()
	}
	list.open = open
	list.up = func() {
		if folder != "" && filepath.Dir(folder) != folder {
			navigate(filepath.Dir(folder))
		}
	}
	up := widget.NewButtonWithIcon("Up", theme.NavigateBackIcon(), list.up)
	goTo := widget.NewButton("Go", func() { navigate(location.Text) })
	toolbar := container.NewBorder(nil, nil, up, goTo, location)
	places := widget.NewSelect([]string{"Home", "Documents", "Downloads"}, func(place string) {
		if home == "" {
			return
		}
		path := home
		if place != "Home" {
			path = filepath.Join(home, place)
		}
		navigate(path)
	})
	places.PlaceHolder = "Locations"
	navigation := container.NewBorder(nil, nil, container.NewHBox(back, forward), nil, ancestors)
	header := container.NewVBox(navigation, toolbar, container.NewBorder(nil, nil, places, showDotfiles, search), widget.NewSeparator())
	if title == "Open a vault" {
		header.Add(vaultOnly)
	}
	cancel := widget.NewButton("Cancel", func() { picker.Hide() })
	actions := container.NewHBox(cancel, choose)
	if foldersOnly {
		actions.Add(useFolder)
	}
	footer := container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, actions, status))
	picker = dialog.NewCustomWithoutButtons(title, container.NewBorder(header, footer, nil, nil, list), parent)
	picker.SetOnClosed(func() { closed = true })
	size := parent.Canvas().Size()
	picker.Resize(fyne.NewSize(minFloat32(780, size.Width-32), minFloat32(520, size.Height-32)))
	navigate(initial)
	picker.Show()
}

func showLocalFile(title string, callback func(string), parent fyne.Window) {
	browseLocal(title, "", false, parent, callback)
}

func showLocalFolder(title, initial string, callback func(string), parent fyne.Window) {
	browseLocal(title, initial, true, parent, callback)
}
