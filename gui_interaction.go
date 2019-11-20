package main

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/gotk3/gotk3/gtk"
)

// #cgo pkg-config: gtk+-3.0
// #include <gtk/gtk.h>
// void createConfirmTags(GtkTextBuffer* buff) {
//   gtk_text_buffer_create_tag(buff, "settingName", "weight", "700", NULL);
//   gtk_text_buffer_create_tag(buff, "warning", "foreground", "red", NULL);
// }
// void createInstallViewTags(GtkTextBuffer* buff) {
//   gtk_text_buffer_create_tag(buff, "cmdMsg", "weight", "700", NULL);
//   gtk_text_buffer_create_tag(buff, "warnMsg", "foreground", "orange", NULL);
//   gtk_text_buffer_create_tag(buff, "errMsg", "foreground", "red", NULL);
// }
import "C"

// Called from initiialization code to populate the list of timezones.
func (mw *mainWindow) setTzs(timezones []string) {
	for _, t := range timezones {
		mw.settings.TzCtrl.Append(t, t)
	}
	mw.settings.TzCtrl.SetActiveID("America/Los_Angeles")
}

// Called from initiialization code to populate the list of disks.
func (mw *mainWindow) setDisks(disks []disk) {
	for _, d := range disks {
		mw.settings.DiskCtrl.Append(d.Path, fmt.Sprintf("%s (%s) - %s bus, %s partition table", d.Path, d.Model, d.Bus, d.PartTabType))
	}
	mw.settings.DiskCtrl.SetActive(0)
}

// Creates a new entry in the debug treeview & populates its value. Called
// from initiialization code.
func (mw *mainWindow) setDebugValue(roots []string, val string) error {
	if n, ok := mw.debugData[roots[0]]; ok {
		return mw.iterSetDebugValue(n, roots[1:], val)
	}
	item := mw.debugModel.Append(nil)
	mw.debugModel.SetValue(item, 0, roots[0])
	mw.debugModel.SetValue(item, 1, val)
	mw.debugInfo.ExpandAll()
	mw.debugData[roots[0]] = &debugInfoNode{
		Name:     roots[0],
		Value:    val,
		Item:     item,
		Children: make(map[string]*debugInfoNode),
	}
	return nil
}

func (mw *mainWindow) iterSetDebugValue(n *debugInfoNode, roots []string, val string) error {
	if len(roots) == 1 {
		item := mw.debugModel.Append(n.Item)
		mw.debugModel.SetValue(item, 0, roots[0])
		mw.debugModel.SetValue(item, 1, val)
		mw.debugInfo.ExpandAll()
		n.Children[roots[0]] = &debugInfoNode{
			Name:     roots[0],
			Value:    val,
			Item:     item,
			Children: make(map[string]*debugInfoNode),
		}
		return nil
	}

	if n, ok := n.Children[roots[0]]; ok {
		return mw.iterSetDebugValue(n, roots[1:], val)
	}
	return os.ErrNotExist
}

// This callback fires when any input on the settings pane
// is changed. If validation is successful, the next button
// is enabled.
func (mw *mainWindow) callbackSettingsTyped() {
	mainPw, _ := mw.settings.PwCtrl.GetText()
	confPw, _ := mw.settings.PwConfirm.GetText()
	host, _ := mw.settings.HostCtrl.GetText()
	user, _ := mw.settings.UserCtrl.GetText()
	scrubChecked := mw.settings.ScrubCheck.GetActive()

	if scrubChecked {
		mw.settings.ScrubWarnLabel.Hide()
	} else {
		mw.settings.ScrubWarnLabel.Show()
	}

	isValid := mainPw != "" && confPw == mainPw && host != "" && user != ""
	if isValid {
		mw.nextBtn.SetSensitive(true)
	} else {
		mw.nextBtn.SetSensitive(false)
	}
}

// This callback is called when the password inputs are changed.
// This sets the red coloring on the label if they dont match,
// and invokes callbackSettingsTyped() to validate remaining fields.
func (mw *mainWindow) callbackPwChanged() {
	mainPw, _ := mw.settings.PwCtrl.GetText()
	confPw, _ := mw.settings.PwConfirm.GetText()
	if mainPw == "" || confPw == "" {
		return
	}

	sc, _ := mw.settings.PwLabel.GetStyleContext()
	if mainPw == confPw {
		sc.AddClass("validPassword")
		sc.RemoveClass("invalidPassword")
	} else {
		sc.AddClass("invalidPassword")
		sc.RemoveClass("validPassword")
	}
	mw.callbackSettingsTyped()
}

// This callback is invoked when the next button is pressed.
//
func (mw *mainWindow) callbackNext() {
	// Advance the current pane, setting next/previous as
	// enabled/disabled as necessary.
	mw.currPane++
	if mw.currPane > len(mw.panes)-1 {
		mw.currPane = len(mw.panes) - 1
		return
	}
	if mw.currPane == 0 {
		mw.prevBtn.SetSensitive(false)
	} else {
		mw.prevBtn.SetSensitive(true)
	}
	if mw.currPane == len(mw.panes)-1 {
		mw.nextBtn.SetSensitive(false)
	} else {
		mw.nextBtn.SetSensitive(true)
	}

	// Swap the current pane for the next one.
	currentPane, err := mw.fullGrid.GetChildAt(0, 1)
	if err != nil {
		fmt.Printf("Failed to get current pane: %v\n", err)
		return
	}
	mw.fullGrid.Remove(currentPane)
	mw.fullGrid.Attach(mw.panes[mw.currPane], 0, 1, 1, 1)

	// Based on these (hardcoded) requirements for the new pane,
	// Style the buttons differently.
	// Also, if pressing next symbolizes some action (ie: install),
	// that action is initiated from here.
	sc, _ := mw.nextBtn.GetStyleContext()
	if mw.currPane == 2 {
		sc.AddClass("danger")
		mw.nextBtn.SetLabel("Install")
		mw.populateConfirmDetails()
	} else if mw.currPane == 3 {
		mw.nextBtn.SetSensitive(false)
		mw.prevBtn.SetSensitive(false)
		sc.RemoveClass("danger")
		mw.doSetupStartInstall()
	} else {
		mw.nextBtn.SetLabel("Next")
		sc.RemoveClass("danger")
		if mw.currPane == 1 {
			mw.nextBtn.SetSensitive(false)
			mw.callbackSettingsTyped()
		}
	}
}

func (mw *mainWindow) doSetupStartInstall() {
	d := getDisk(mw.settings.DiskCtrl.GetActiveText())
	p, err := mw.settings.PwCtrl.GetText()
	if err != nil {
		fmt.Printf("Failed to read password: %v\n", err)
		return
	}
	u, err := mw.settings.UserCtrl.GetText()
	if err != nil {
		fmt.Printf("Failed to read username: %v\n", err)
		return
	}
	h, err := mw.settings.HostCtrl.GetText()
	if err != nil {
		fmt.Printf("Failed to read username: %v\n", err)
		return
	}
	scrub := mw.settings.ScrubCheck.GetActive()

	state := installState{
		InstallDevice: &d,
		Pw:            p,
		User:          u,
		Host:          h,
		Scrub:         scrub,
		Tz:            mw.settings.TzCtrl.GetActiveText(),
	}

	for _, pkg := range mw.settings.Pkgs {
		if pkg.checkbox.GetActive() {
			state.OptionalPkgs = append(state.OptionalPkgs, pkg.Name)
		}
	}

	go mw.doInstallRoutine(state)
}

// This callback is invoked when the previous button is pressed.
func (mw *mainWindow) callbackPrev() {
	mw.currPane--
	if mw.currPane < 0 {
		mw.currPane = 0
		return
	}
	if mw.currPane == 0 {
		mw.prevBtn.SetSensitive(false)
	} else {
		mw.prevBtn.SetSensitive(true)
	}
	if mw.currPane == len(mw.panes)-1 {
		mw.nextBtn.SetSensitive(false)
	} else {
		mw.nextBtn.SetSensitive(true)
	}

	currentPane, err := mw.fullGrid.GetChildAt(0, 1)
	if err != nil {
		fmt.Printf("Failed to get current pane: %v\n", err)
		return
	}

	mw.fullGrid.Remove(currentPane)
	mw.fullGrid.Attach(mw.panes[mw.currPane], 0, 1, 1, 1)
	mw.nextBtn.SetLabel("Next")
	sc, err := mw.nextBtn.GetStyleContext()
	if err != nil {
		fmt.Printf("nextBtn.GetStyleContext() failed: %v\n", err)
		return
	}
	sc.RemoveClass("danger")

	if mw.currPane == 1 {
		mw.callbackSettingsTyped()
	}
}

type textviewStyleSelection struct {
	Start, End int
	Class      string
}

func (mw *mainWindow) populateConfirmDetails() {
	ttt, _ := gtk.TextTagTableNew()
	textBuffer, err := gtk.TextBufferNew(ttt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TextBufferNew() failed: %v\n", err)
		return
	}
	mw.confirmView.SetBuffer(textBuffer)
	C.createConfirmTags((*C.GtkTextBuffer)(unsafe.Pointer(textBuffer.Native())))

	var outText string
	var styles []textviewStyleSelection
	writeStyled := func(text, class string) {
		if class != "" {
			styles = append(styles, textviewStyleSelection{
				Start: len(outText),
				End:   len(outText) + len(text),
				Class: class,
			})
		}
		outText += text
	}

	hn, _ := mw.settings.HostCtrl.GetText()
	writeStyled("Hostname: ", "settingName")
	writeStyled(hn, "")
	writeStyled("\n", "")
	un, _ := mw.settings.UserCtrl.GetText()
	writeStyled("Username: ", "settingName")
	writeStyled(un, "")
	writeStyled("\n", "")
	writeStyled("Timezone: ", "settingName")
	writeStyled(mw.settings.TzCtrl.GetActiveText(), "")
	writeStyled("\n\n", "")

	d := getDisk(mw.settings.DiskCtrl.GetActiveText())
	writeStyled("Install to:\n  Path: ", "settingName")
	writeStyled(d.Path+"\n", "")
	writeStyled("  Name: ", "settingName")
	if d.Label != "" {
		writeStyled(d.Label+"\n        ", "")
	}
	writeStyled(d.Model+" ("+d.Serial+")", "")
	writeStyled("\n", "")
	writeStyled("  Capacity: ", "settingName")
	writeStyled(byteCountDecimal(int64(d.NumBlocks*512))+"\n", "")
	writeStyled("  UUID: ", "settingName")
	writeStyled(d.PartUUID, "")
	writeStyled("\n", "")
	writeStyled("  Partitions: ", "settingName")
	writeStyled(fmt.Sprintf("%d read from %s table", len(d.Partitions), d.PartTabType), "")
	writeStyled("\n", "")
	for i, part := range d.Partitions {
		writeStyled(fmt.Sprintf("   %2d: ", i), "settingName")
		writeStyled(fmt.Sprintf("%s filesystem on %s\n", part.FS, part.Name), "")
		writeStyled(fmt.Sprintf("      Filesystem UUID: %s\n", part.FsUUID), "")
		writeStyled(fmt.Sprintf("      Partition UUID: %s\n", part.PartUUID), "")
	}
	writeStyled("  WARNING: Any existing data on this disk will be lost.\n", "warning")

	if mw.settings.ScrubCheck.GetActive() {
		writeStyled("  Zeros will be written to existing partition (scrubbing) before formatting.\n", "")
	} else {
		writeStyled("  WARNING: Encrypted partition will not be scrubbed. This may reveal information\n", "warning")
		writeStyled("  about the usage patterns of your system if your disk is examined.\n", "warning")
	}

	p, _ := mw.settings.PwCtrl.GetText()
	// If I find a nice entropy evaluation library, we should use that instead.
	// I am fully aware that 8 chars is both too short, and a poor estimate of entropy.
	if len(p) <= 8 {
		writeStyled("\nPassword:\n", "settingName")
		writeStyled("  Your password is super short, consider revising.\n", "warning")
		writeStyled("  This directly affects the confidentiality/integrity of your data.\n", "warning")
	}

	if len(mw.settings.Pkgs) > 0 {
		writeStyled("\nExtra packages:\n", "settingName")
		for _, pkg := range mw.settings.Pkgs {
			if pkg.checkbox.GetActive() {
				writeStyled("  "+pkg.DisplayName+" ("+pkg.Version+")\n", "")
			}
		}
	}

	textBuffer.SetText(outText)
	for _, t := range styles {
		if t.Class != "" {
			textBuffer.ApplyTagByName(t.Class, textBuffer.GetIterAtOffset(t.Start), textBuffer.GetIterAtOffset(t.End))
		}
	}
}

func (mw *mainWindow) callbackWindowDestroy() {
	gtk.MainQuit()
}

func (mw *mainWindow) mainLoop() {
	mw.win.SetDefaultSize(800, 600)
	mw.win.ShowAll()

	if !*debugMode {
		parent, err := mw.debugInfo.GetParent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "debugInfo.GetParent() failed: %v\n", err)
			return
		}
		parent.Hide()
	}

	ttt, err := gtk.TextTagTableNew()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TextTagTableNew() failed: %v\n", err)
		return
	}
	textBuffer, err := gtk.TextBufferNew(ttt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TextBufferNew() failed: %v\n", err)
		return
	}
	mw.installView.SetBuffer(textBuffer)
	C.createInstallViewTags((*C.GtkTextBuffer)(unsafe.Pointer(textBuffer.Native())))
	go mw.processProgressEventsRoutine(textBuffer, mw.installViewScroll)

	gtk.Main()
}
