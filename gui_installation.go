package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// #cgo pkg-config: gtk+-3.0
// #include <gtk/gtk.h>
// void textbuffDelete(GtkTextBuffer *buff, GtkTextIter *cursor, GtkTextIter* end) {
//   gtk_text_buffer_delete(buff, cursor, end);
// }
import "C"

var steps = []InstallStep{
	&PartitionStep{},
	&CopyStep{},
	&ConfigureStep{},
	&CleanupStep{},
}

type InstallStep interface {
	Run(chan progressUpdate, *installState) error
	Name() string
}

type installState struct {
	InstallDevice *disk
	Pw, User      string
	Tz, Host      string
	Scrub         bool
	Autologin     bool

	OptionalPkgs []string
}

type progressUpdate struct {
	Percent         int
	TransistionStep int

	CmdMsg     string
	InfoMsg    string
	WarnMsg    string
	ErrMsg     string
	IsProgress bool
}

func progressInfo(updateChan chan progressUpdate, fmtStr string, args ...interface{}) {
	updateChan <- progressUpdate{
		InfoMsg: fmt.Sprintf("  "+fmtStr, args...),
	}
}

func runCmd(updateChan chan progressUpdate, logPrefix, cmd string, args ...string) error {
	e := exec.Command(cmd, args...)
	progressInfo(updateChan, "%s%s\n", logPrefix, args)
	out, err := e.CombinedOutput()
	if len(out) > 4 {
		progressInfo(updateChan, "  Output: %q\n", string(out))
	}
	return err
}

type cmdInteractiveWriter struct {
	updateChan chan progressUpdate
	logPrefix  string
	IsErr      bool
	IsProgress bool
}

func (c *cmdInteractiveWriter) Write(in []byte) (int, error) {
	for _, line := range strings.Split(string(in), "\n") {
		line = strings.Trim(line, " \r\n")
		if len(line) < 2 {
			continue
		}

		var out progressUpdate
		if c.IsErr {
			out = progressUpdate{
				WarnMsg: fmt.Sprintf("  %s%s\n", c.logPrefix, line),
			}
		} else {
			out = progressUpdate{
				InfoMsg: fmt.Sprintf("  %s%s\n", c.logPrefix, line),
			}
		}
		out.IsProgress = c.IsProgress
		c.updateChan <- out
	}
	return len(in), nil
}

func runCmdInteractive(updateChan chan progressUpdate, logPrefix, cmd string, args ...string) error {
	e := exec.Command(cmd, args...)
	progressInfo(updateChan, "%s%s\n", logPrefix, args)

	e.Stdout = &cmdInteractiveWriter{
		updateChan: updateChan,
		logPrefix:  logPrefix,
	}
	e.Stderr = &cmdInteractiveWriter{
		updateChan: updateChan,
		logPrefix:  logPrefix,
		IsErr:      true,
	}

	return e.Run()
}

func (mw *mainWindow) processProgressEventsRoutine(textBuffer *gtk.TextBuffer, scrollWindow *gtk.ScrolledWindow) {
	var outText string
	sync := make(chan bool)

	for evt := range mw.progressUpdate {
		if evt.TransistionStep > 0 {
			glib.IdleAdd(func() {
				for _, lab := range mw.stepLabels {
					sc, _ := lab.GetStyleContext()
					sc.RemoveClass("active-progress-label")
				}
				if evt.TransistionStep-1 < len(mw.stepLabels) {
					sc, _ := mw.stepLabels[evt.TransistionStep-1].GetStyleContext()
					sc.AddClass("active-progress-label")
				}
				sync <- true
			})
			<-sync
		}

		if evt.IsProgress {
			// Cut out up to the 2nd-last newline.
			i := strings.LastIndex(outText, "\n")
			if i > 0 {
				i = strings.LastIndex(outText[:i], "\n")
				if i > 0 {
					i++
					outText = outText[:i]
					glib.IdleAdd(func() {
						curs := textBuffer.GetIterAtOffset(i)
						end := textBuffer.GetEndIter()
						C.textbuffDelete((*C.GtkTextBuffer)(unsafe.Pointer(textBuffer.Native())), (*C.GtkTextIter)(unsafe.Pointer(curs)), (*C.GtkTextIter)(unsafe.Pointer(end)))
						sync <- true
					})
					<-sync
				}
			}
		}

		if evt.CmdMsg != "" {
			outText += evt.CmdMsg
			glib.IdleAdd(func() {
				textBuffer.InsertAtCursor(evt.CmdMsg)
				textBuffer.ApplyTagByName("cmdMsg", textBuffer.GetIterAtOffset(len(outText)-len(evt.CmdMsg)), textBuffer.GetIterAtOffset(len(outText)))
				sync <- true
			})
			<-sync
		}
		if evt.ErrMsg != "" {
			outText += evt.ErrMsg
			glib.IdleAdd(func() {
				textBuffer.InsertAtCursor(evt.ErrMsg)
				textBuffer.ApplyTagByName("errMsg", textBuffer.GetIterAtOffset(len(outText)-len(evt.ErrMsg)), textBuffer.GetIterAtOffset(len(outText)))
				sync <- true
			})
			<-sync
		}
		if evt.WarnMsg != "" {
			outText += evt.WarnMsg
			glib.IdleAdd(func() {
				textBuffer.InsertAtCursor(evt.WarnMsg)
				textBuffer.ApplyTagByName("warnMsg", textBuffer.GetIterAtOffset(len(outText)-len(evt.WarnMsg)), textBuffer.GetIterAtOffset(len(outText)))
				sync <- true
			})
			<-sync
		}
		if evt.InfoMsg != "" {
			outText += evt.InfoMsg
			glib.IdleAdd(func() {
				textBuffer.InsertAtCursor(evt.InfoMsg)
				sync <- true
			})
			<-sync
		}

		if !evt.IsProgress {
			glib.IdleAdd(func() {
				adj := scrollWindow.GetVAdjustment()
				adj.SetValue(adj.GetUpper())
				sync <- true
			})
			<-sync
		}
	}
}

func (mw *mainWindow) doInstallRoutine(state installState) {
	for i, step := range steps {
		mw.progressUpdate <- progressUpdate{
			CmdMsg:          fmt.Sprintf("Starting %s\n", step.Name()),
			TransistionStep: i + 1,
		}
		if err := step.Run(mw.progressUpdate, &state); err != nil {
			fmt.Fprintf(os.Stderr, "Step %d failed: %v\n", i+1, err)
			mw.progressUpdate <- progressUpdate{
				ErrMsg: fmt.Sprintf("\nError!: %v\n", err),
			}
			return
		}
		mw.progressUpdate <- progressUpdate{
			CmdMsg: fmt.Sprintf("Finished %s\n", step.Name()),
		}
	}

	mw.progressUpdate <- progressUpdate{
		CmdMsg: "\nInstallation of TwitchyLinux has finished!!\nYou may now power-cycle your computer & remove installation media.\n",
	}
}
