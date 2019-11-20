package main

import (
	"github.com/gotk3/gotk3/gtk"
)

func createTextColumn(title string, id int) *gtk.TreeViewColumn {
	cellRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		panic(err)
	}
	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellRenderer, "text", id)
	if err != nil {
		panic(err)
	}

	return column
}

const styling = `

.view.debugTree {
  background-color: #F2F1F0;
}

#confirmInfo {
  background-color: #F2F1F0;
}

.danger > label {
	color: #E23243;
	font-weight: 800;
}

.invalidPassword {
	color: #CC1111;
	font-weight: bold;
}

.active-progress-label {
	font-weight: bold;
	font-size: 135%;
}
`
