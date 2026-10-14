//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"lazarusbinder/internal/bundle"
	"lazarusbinder/internal/icons"
	"lazarusbinder/internal/winui"
	"os"
	"path/filepath"
	"strings"
)

//go:embed launcher.exe
var launcher []byte

//go:embed launcher-admin.exe
var adminLauncher []byte

const exeFilter = "Windows programs (*.exe)\x00*.exe\x00\x00"
const iconFilter = "Windows icons (*.ico)\x00*.ico\x00\x00"
const resourceFilter = "Windows programs and libraries\x00*.exe;*.dll\x00\x00"

func main() {
	w, err := winui.New("LazarusBinder | bundle programs and manage icons", 880, 620)
	if err != nil {
		winui.Fatal(err)
		return
	}
	w.Title("LazarusBinder", 28, 20, 430, 42)
	w.Label("Two programs. One EXE. Your icon.", 29, 67, 760, 26)
	var nav []uintptr
	for i, label := range []string{"01  Bundle programs", "02  Set bundle icon", "03  Extract icons"} {
		n := i
		nav = append(nav, w.Button(label, 28+i*278, 112, 268, 40, func() { w.ShowPage(n) }))
	}
	status := w.Label("Ready. All work stays on this computer.", 28, 571, 820, 38)
	var jobs []uintptr
	busy := false
	run := func(message string, work func() error, done func()) {
		if busy {
			return
		}
		busy = true
		w.SetText(status, message)
		for _, h := range append(append([]uintptr{}, jobs...), nav...) {
			w.Enable(h, false)
		}
		go func() {
			e := work()
			w.Post(func() {
				busy = false
				for _, h := range append(append([]uintptr{}, jobs...), nav...) {
					w.Enable(h, true)
				}
				if e != nil {
					w.SetText(status, "Operation failed. Your inputs were kept.")
					w.Alert("Could not complete operation", e.Error())
				} else {
					w.SetText(status, "Done.")
					if done != nil {
						done()
					}
				}
			})
		}()
	}
	choose := func(title, filter, ext string) string {
		path, e := w.FileDialog(title, filter, ext, "", false)
		if e != nil {
			w.Alert("Choose a file", e.Error())
		}
		return path
	}
	save := func(title, filter, ext, name string) string {
		path, e := w.FileDialog(title, filter, ext, name, true)
		if e != nil {
			w.Alert("Save a file", e.Error())
		}
		return path
	}

	w.Page(0)
	w.Heading("Bundle two standalone EXEs", 28, 177, 700, 30)
	w.Label("The final combined EXE can request administrator access for both embedded programs.", 28, 213, 815, 26)
	w.Label("Program 1", 28, 257, 110, 26)
	first := w.Edit("", 143, 252, 566, 32, true)
	jobs = append(jobs, w.Button("Browse...", 721, 251, 130, 34, func() {
		if path := choose("Choose the first program", exeFilter, "exe"); path != "" {
			w.SetText(first, path)
		}
	}))
	w.Label("Program 2", 28, 303, 110, 26)
	second := w.Edit("", 143, 298, 566, 32, true)
	jobs = append(jobs, w.Button("Browse...", 721, 297, 130, 34, func() {
		if path := choose("Choose the second program", exeFilter, "exe"); path != "" {
			w.SetText(second, path)
		}
	}))
	w.Label("Bundle name", 28, 349, 110, 26)
	title := w.Edit("My program bundle", 143, 344, 708, 32, false)
	w.Label("Icon (optional)", 28, 395, 110, 26)
	icon := w.Edit("", 143, 390, 430, 32, true)
	preview := w.IconControl(594, 383, 64)
	jobs = append(jobs, w.Button("Choose ICO", 721, 389, 130, 34, func() {
		if path := choose("Choose the bundle icon", iconFilter, "ico"); path != "" {
			if _, e := icons.ReadICO(path); e != nil {
				w.Alert("Invalid icon", e.Error())
				return
			}
			w.SetText(icon, path)
			w.Preview(preview, path)
		}
	}))
	jobs = append(jobs, w.Button("Clear", 721, 430, 130, 30, func() { w.SetText(icon, ""); w.Preview(preview, "") }))
	requireAdmin := w.Checkbox("Require administrator for the combined EXE", 143, 441, 440, 28)
	w.SetChecked(requireAdmin, true)
	wait := w.Checkbox("Start program 2 after program 1 exits", 143, 473, 440, 26)
	jobs = append(jobs, requireAdmin, wait, title)
	w.Label("Use standalone EXEs. Nearby DLLs, folders and external runtimes are not bundled.", 28, 502, 815, 22)
	jobs = append(jobs, w.Button("Create combined EXE", 579, 525, 272, 36, func() {
		paths := [2]string{w.Text(first), w.Text(second)}
		for _, p := range paths {
			if e := bundle.ValidateEXE(p); e != nil {
				w.Alert("Choose two programs", e.Error())
				return
			}
		}
		output := save("Save the combined EXE", exeFilter, "exe", "Combined.exe")
		if output == "" {
			return
		}
		bundleTitle, iconPath, sequence := w.Text(title), w.Text(icon), w.Checked(wait)
		options := bundle.Options{Title: bundleTitle, WaitForFirst: sequence, RequireAdmin: w.Checked(requireAdmin)}
		template := launcher
		if options.RequireAdmin {
			template = adminLauncher
		}
		var setter func(string) error
		if iconPath != "" {
			setter = func(path string) error { return icons.SetFile(path, iconPath) }
		}
		run("Building and compressing the two programs...", func() error { return bundle.Build(template, paths, options, output, setter) }, func() {
			w.SetText(status, "Created "+output)
			w.Info("Bundle created", "Saved:\n"+output+"\n\nDouble-click the new EXE to start both programs.")
		})
	}))

	w.Page(1)
	w.Heading("Give your bundle a new icon", 28, 177, 700, 30)
	w.Label("Choose a LazarusBinder bundle and save a new copy with a different Windows icon.", 28, 213, 815, 30)
	w.Label("Bundle EXE", 28, 270, 110, 26)
	bound := w.Edit("", 143, 265, 566, 32, true)
	jobs = append(jobs, w.Button("Browse...", 721, 264, 130, 34, func() {
		if path := choose("Choose a LazarusBinder bundle", exeFilter, "exe"); path != "" {
			a, e := bundle.Open(path)
			if e != nil {
				w.Alert("Choose a LazarusBinder bundle", e.Error())
				return
			}
			a.Close()
			w.SetText(bound, path)
		}
	}))
	w.Label("New icon", 28, 326, 110, 26)
	newIcon := w.Edit("", 143, 321, 566, 32, true)
	newPreview := w.IconControl(143, 380, 64)
	jobs = append(jobs, w.Button("Choose ICO", 721, 320, 130, 34, func() {
		if path := choose("Choose the new icon", iconFilter, "ico"); path != "" {
			if _, e := icons.ReadICO(path); e != nil {
				w.Alert("Invalid icon", e.Error())
				return
			}
			w.SetText(newIcon, path)
			w.Preview(newPreview, path)
		}
	}))
	w.Label("ICO files can include several sizes, so your icon stays sharp in Explorer.\nThe copy keeps both embedded programs intact.", 235, 385, 605, 68)
	w.Label("Extract an icon in the next section, then reuse that ICO file here.", 28, 485, 815, 35)
	jobs = append(jobs, w.Button("Save EXE with new icon", 579, 525, 272, 36, func() {
		source, ico := w.Text(bound), w.Text(newIcon)
		if _, e := icons.ReadICO(ico); e != nil {
			w.Alert("Choose an icon", e.Error())
			return
		}
		output := save("Save the bundle with its new icon", exeFilter, "exe", "Combined-new-icon.exe")
		if output == "" {
			return
		}
		run("Updating the bundle icon...", func() error {
			return bundle.Retouch(source, output, func(path string) error { return icons.SetFile(path, ico) })
		}, func() {
			w.SetText(status, "Created "+output)
			w.Info("Icon applied", "Saved:\n"+output+"\n\nBoth bundled programs were preserved.")
		})
	}))

	w.Page(2)
	w.Heading("Extract icons from an EXE or DLL", 28, 177, 760, 30)
	w.Label("Reads embedded icon resources without running the selected program.", 28, 213, 815, 26)
	w.Label("Source file", 28, 270, 110, 26)
	source := w.Edit("", 143, 265, 566, 32, true)
	var groups []icons.Group
	var groupCombo, groupPreview, summary uintptr
	updatePreview := func() {
		i := w.Selection(groupCombo)
		if i < 0 || i >= len(groups) {
			return
		}
		g := groups[i]
		w.SetText(summary, g.Label+"\nSizes: "+g.Sizes)
		temp, e := os.CreateTemp("", "lazarusbinder-preview-*.ico")
		if e != nil {
			return
		}
		path := temp.Name()
		_, e = temp.Write(g.Data)
		temp.Close()
		defer os.Remove(path)
		if e == nil {
			w.Preview(groupPreview, path)
		}
	}
	jobs = append(jobs, w.Button("Load icons", 721, 264, 130, 34, func() {
		path := choose("Choose an EXE or DLL", resourceFilter, "exe")
		if path == "" {
			return
		}
		var found []icons.Group
		run("Reading embedded icon resources...", func() error { var e error; found, e = icons.Extract(path); return e }, func() {
			groups = found
			w.SetText(source, path)
			items := make([]string, len(groups))
			for i, g := range groups {
				items[i] = g.Label
			}
			w.SetItems(groupCombo, items)
			updatePreview()
			w.SetText(status, fmt.Sprintf("Found %d icon group(s).", len(groups)))
		})
	}))
	w.Label("Icon group", 28, 326, 110, 26)
	groupCombo = w.Combo(143, 321, 708, 220, updatePreview)
	groupPreview = w.IconControl(143, 387, 64)
	summary = w.Label("Load a program to see its embedded icons.", 236, 390, 605, 94)
	w.Label("Exported ICO files keep all embedded resolutions and color depths.", 28, 489, 815, 32)
	jobs = append(jobs, w.Button("Export all groups", 293, 525, 272, 36, func() {
		if len(groups) == 0 {
			w.Alert("Load a program", "Choose an EXE or DLL with embedded icons first.")
			return
		}
		path := save("Choose a base name for the icon folder", iconFilter, "ico", "Extracted-icons.ico")
		if path == "" {
			return
		}
		folder := strings.TrimSuffix(path, filepath.Ext(path)) + "-groups"
		copyGroups := append([]icons.Group(nil), groups...)
		run("Exporting all icon groups...", func() error {
			if e := os.Mkdir(folder, 0700); e != nil {
				return e
			}
			for i, g := range copyGroups {
				if e := os.WriteFile(filepath.Join(folder, fmt.Sprintf("icon-%03d.ico", i+1)), g.Data, 0600); e != nil {
					os.RemoveAll(folder)
					return e
				}
			}
			return nil
		}, func() {
			w.SetText(status, "Exported "+folder)
			w.Info("Icons exported", "Saved all icon groups in:\n"+folder)
		})
	}))
	jobs = append(jobs, w.Button("Export selected ICO", 579, 525, 272, 36, func() {
		i := w.Selection(groupCombo)
		if i < 0 || i >= len(groups) {
			w.Alert("Load a program", "Choose an EXE or DLL with embedded icons first.")
			return
		}
		path := save("Save the selected icon", iconFilter, "ico", "Extracted-icon.ico")
		if path == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".ico") {
			w.Alert("Save an ICO file", "The exported filename must end in .ico.")
			return
		}
		data := groups[i].Data
		run("Exporting the selected icon...", func() error { return os.WriteFile(path, data, 0600) }, func() { w.SetText(status, "Exported "+path) })
	}))
	w.ShowPage(0)
	w.Run()
}
