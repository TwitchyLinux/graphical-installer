package main

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"unicode"
)

const tzBase = "/usr/share/zoneinfo"

var timezones []string

func readTimezoneInfo(mw *mainWindow) {
	readTzDir("")
	mw.setDebugValue([]string{"aux"}, "")
	mw.setDebugValue([]string{"aux", "num_timezones"}, fmt.Sprint(len(timezones)))
	mw.setTzs(timezones)
}

func readTzDir(p string) {
	fs, _ := ioutil.ReadDir(path.Join(tzBase, p))
	for _, f := range fs {
		if strings.HasPrefix(f.Name(), "posix") || strings.HasPrefix(f.Name(), "right") || !unicode.Is(unicode.Upper, rune(f.Name()[0])) {
			continue
		}

		if f.IsDir() {
			readTzDir(path.Join(p, f.Name()))
		} else {
			timezones = append(timezones, path.Join(p, f.Name()))
		}
	}
}
