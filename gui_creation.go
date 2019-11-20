package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const (
	winTitle = "TwitchyLinux Installer"
)

type debugInfoNode struct {
	Name, Value string
	Item        *gtk.TreeIter
	Children    map[string]*debugInfoNode
}

type mainWindow struct {
	win     *gtk.Window
	cssProv *gtk.CssProvider

	fullGrid         *gtk.Grid
	currPane         int
	panes            map[int]*gtk.Grid
	nextBtn, prevBtn *gtk.Button
	abortBtn         *gtk.Button
	versionLab       *gtk.Label

	debugInfo  *gtk.TreeView
	debugModel *gtk.TreeStore
	debugData  map[string]*debugInfoNode

	settings struct {
		HostCtrl *gtk.Entry
		UserCtrl *gtk.Entry

		TzCtrl   *gtk.ComboBoxText
		DiskCtrl *gtk.ComboBoxText

		PwCtrl    *gtk.Entry
		PwConfirm *gtk.Entry
		PwLabel   *gtk.Label

		ScrubCheck     *gtk.CheckButton
		ScrubWarnLabel *gtk.Label

		PkgChecksBox *gtk.Box
		Pkgs         []optPkg
	}

	confirmView       *gtk.TextView
	installView       *gtk.TextView
	installViewScroll *gtk.ScrolledWindow
	stepLabels        []*gtk.Label
	progressUpdate    chan progressUpdate
}

func makeMainWindow() (*mainWindow, error) {
	mw := mainWindow{
		debugData:      make(map[string]*debugInfoNode),
		panes:          make(map[int]*gtk.Grid),
		progressUpdate: make(chan progressUpdate, 2),
	}

	b, err := gtk.BuilderNewFromFile("layout.glade")
	if err != nil {
		return nil, err
	}

	obj, err := b.GetObject("window")
	if err != nil {
		return nil, errors.New("couldnt find window in glade file")
	}
	if w, ok := obj.(*gtk.Window); ok {
		mw.win = w
	}

	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return nil, err
	}
	mw.cssProv, err = gtk.CssProviderNew()
	if err != nil {
		return nil, err
	}
	gtk.AddProviderForScreen(screen, mw.cssProv, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	if err := mw.cssProv.LoadFromData(styling); err != nil {
		return nil, err
	}

	mw.win.SetTitle(winTitle)
	mw.win.Connect("destroy", mw.callbackWindowDestroy)

	return &mw, mw.build(b)
}

func (mw *mainWindow) build(b *gtk.Builder) error {
	// Buttons on the bottom.
	obj, err := b.GetObject("abortBtn")
	if err != nil {
		return errors.New("couldnt find abortBtn")
	}
	mw.abortBtn = obj.(*gtk.Button)
	mw.abortBtn.Connect("clicked", mw.callbackWindowDestroy)
	obj, err = b.GetObject("nextBtn")
	if err != nil {
		return errors.New("couldnt find nextBtn")
	}
	mw.nextBtn = obj.(*gtk.Button)
	mw.nextBtn.Connect("clicked", mw.callbackNext)
	obj, err = b.GetObject("previousBtn")
	if err != nil {
		return errors.New("couldnt find prevBtn")
	}
	mw.prevBtn = obj.(*gtk.Button)
	mw.prevBtn.Connect("clicked", mw.callbackPrev)
	mw.prevBtn.SetSensitive(false)

	// Panes
	obj, err = b.GetObject("contentGrid")
	if err != nil {
		return errors.New("couldnt find contentGrid")
	}
	mw.panes[0] = obj.(*gtk.Grid)
	if err := mw.loadPaneReference(b, "contentGrid_settings", "fullGrid_settings", 1); err != nil {
		return err
	}
	if err := mw.loadPaneReference(b, "contentGrid_confirmation", "fullGrid_confirmation", 2); err != nil {
		return err
	}
	if err := mw.loadPaneReference(b, "contentGrid_progress", "fullGrid_progress", 3); err != nil {
		return err
	}

	obj, err = b.GetObject("fullGrid")
	if err != nil {
		return errors.New("couldnt find fullGrid")
	}
	mw.fullGrid = obj.(*gtk.Grid)

	obj, err = b.GetObject("confirmInfo")
	if err != nil {
		return errors.New("couldnt find confirmInfo")
	}
	mw.confirmView = obj.(*gtk.TextView)

	obj, err = b.GetObject("outputProgressText")
	if err != nil {
		return errors.New("couldnt find outputProgressText")
	}
	mw.installView = obj.(*gtk.TextView)
	obj, err = b.GetObject("outputProgressScroller")
	if err != nil {
		return errors.New("couldnt find outputProgressScroller")
	}
	mw.installViewScroll = obj.(*gtk.ScrolledWindow)
	for i, _ := range steps {
		obj, err = b.GetObject(fmt.Sprintf("progressstep_%d", i+1))
		if err != nil {
			return errors.New("couldnt find progress step")
		}
		mw.stepLabels = append(mw.stepLabels, obj.(*gtk.Label))
	}

	obj, err = b.GetObject("versionLabel")
	if err != nil {
		return errors.New("couldnt find versionLabel")
	}
	mw.versionLab = obj.(*gtk.Label)
	mw.versionLab.SetText("TwitchyLinux " + *version)

	return mw.makeDebugInfo(b)
}

func (mw *mainWindow) loadPaneReference(b *gtk.Builder, paneID, parentID string, paneIndex int) error {
	obj, err := b.GetObject(paneID)
	if err != nil {
		return fmt.Errorf("couldnt find %q", paneID)
	}
	mw.panes[paneIndex] = obj.(*gtk.Grid)
	obj, err = b.GetObject(parentID)
	if err != nil {
		return fmt.Errorf("couldnt find %q", parentID)
	}
	obj.(*gtk.Grid).Remove(mw.panes[paneIndex])
	return nil
}

func (mw *mainWindow) makeDebugInfo(b *gtk.Builder) error {
	obj, err := b.GetObject("debugInfo")
	if err != nil {
		return errors.New("couldnt find debugInfo")
	}
	mw.debugInfo = obj.(*gtk.TreeView)
	mw.debugInfo.AppendColumn(createTextColumn("Device", 0))
	mw.debugInfo.AppendColumn(createTextColumn("Information", 1))

	mw.debugModel, err = gtk.TreeStoreNew(glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		return err
	}
	mw.debugInfo.SetModel(mw.debugModel)
	selection, err := mw.debugInfo.GetSelection()
	if err != nil {
		return err
	}
	selection.SetMode(gtk.SELECTION_NONE)

	return mw.makeSettingsPane(b)
}

type optPkg struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayName string `json:"display_name"`
	checkbox    *gtk.CheckButton
}

func (mw *mainWindow) makePkgChooserCheckboxes(b *gtk.Builder) error {
	obj, err := b.GetObject("pkgChooserBox")
	if err != nil {
		return errors.New("couldnt find pkgChooserBox")
	}
	mw.settings.PkgChecksBox = obj.(*gtk.Box)

	pkgDirs, err := ioutil.ReadDir("/deb-pkgs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read /deb-pkgs: %v\n", err)
		return nil
	}
	for _, dir := range pkgDirs {
		if !dir.IsDir() {
			continue
		}
		meta, err := ioutil.ReadFile(path.Join("/deb-pkgs", dir.Name(), "meta.json"))
		if err != nil {
			return err
		}
		var out optPkg
		if err := json.Unmarshal(meta, &out); err != nil {
			return fmt.Errorf("%s/meta.json: %v", dir.Name(), err)
		}
		out.checkbox, err = gtk.CheckButtonNewWithLabel(out.DisplayName + "(" + out.Version + ")")
		if err != nil {
			return err
		}
		mw.settings.PkgChecksBox.Add(out.checkbox)
		out.checkbox.Show()
		mw.settings.Pkgs = append(mw.settings.Pkgs, out)
	}

	return nil
}

func (mw *mainWindow) makeSettingsPane(b *gtk.Builder) error {
	obj, err := b.GetObject("timezoneCombo")
	if err != nil {
		return errors.New("couldnt find timezoneCombo")
	}
	mw.settings.TzCtrl = obj.(*gtk.ComboBoxText)
	obj, err = b.GetObject("installDiskCombo")
	if err != nil {
		return errors.New("couldnt find installDiskCombo")
	}
	mw.settings.DiskCtrl = obj.(*gtk.ComboBoxText)

	obj, err = b.GetObject("passwordInput")
	if err != nil {
		return errors.New("couldnt find passwordInput")
	}
	mw.settings.PwCtrl = obj.(*gtk.Entry)
	mw.settings.PwCtrl.Connect("changed", mw.callbackPwChanged)
	obj, err = b.GetObject("confirmPasswordInput")
	if err != nil {
		return errors.New("couldnt find confirmPasswordInput")
	}
	mw.settings.PwConfirm = obj.(*gtk.Entry)
	mw.settings.PwConfirm.Connect("changed", mw.callbackPwChanged)

	obj, err = b.GetObject("passwordLabel")
	if err != nil {
		return errors.New("couldnt find passwordLabel")
	}
	mw.settings.PwLabel = obj.(*gtk.Label)

	obj, err = b.GetObject("hostnameInput")
	if err != nil {
		return errors.New("couldnt find hostnameInput")
	}
	mw.settings.HostCtrl = obj.(*gtk.Entry)
	mw.settings.HostCtrl.Connect("changed", mw.callbackSettingsTyped)

	obj, err = b.GetObject("usernameInput")
	if err != nil {
		return errors.New("couldnt find usernameInput")
	}
	mw.settings.UserCtrl = obj.(*gtk.Entry)
	mw.settings.UserCtrl.Connect("changed", mw.callbackSettingsTyped)

	obj, err = b.GetObject("clearDiskCheck")
	if err != nil {
		return errors.New("couldnt find clearDiskCheck")
	}
	mw.settings.ScrubCheck = obj.(*gtk.CheckButton)
	mw.settings.ScrubCheck.Connect("toggled", mw.callbackSettingsTyped)

	obj, err = b.GetObject("clearDiskWarning")
	if err != nil {
		return errors.New("couldnt find clearDiskWarning")
	}
	mw.settings.ScrubWarnLabel = obj.(*gtk.Label)

	return mw.makePkgChooserCheckboxes(b)
}
