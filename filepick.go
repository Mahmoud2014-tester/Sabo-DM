package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	d "github.com/lxn/walk/declarative"
)

// filesModel lists the files inside a torrent with a check box for each.
type filesModel struct {
	walk.TableModelBase
	files    []torrentFile
	checked  []bool
	rows     []int // indexes into files; padding files are hidden
	onChange func()
}

func newFilesModel(m *torrentMeta) *filesModel {
	fm := &filesModel{files: m.Files, checked: make([]bool, len(m.Files))}
	for i, f := range m.Files {
		fm.checked[i] = true
		if !f.Pad {
			fm.rows = append(fm.rows, i)
		}
	}
	return fm
}

func (m *filesModel) RowCount() int { return len(m.rows) }

func (m *filesModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return nil
	}
	f := m.files[m.rows[row]]
	if col == 0 {
		return f.Path
	}
	return humanBytes(float64(f.Size))
}

func (m *filesModel) Checked(row int) bool {
	return row >= 0 && row < len(m.rows) && m.checked[m.rows[row]]
}

func (m *filesModel) SetChecked(row int, c bool) error {
	if row >= 0 && row < len(m.rows) {
		m.checked[m.rows[row]] = c
	}
	if m.onChange != nil {
		m.onChange()
	}
	return nil
}

func (m *filesModel) setAll(c bool) {
	for _, r := range m.rows {
		m.checked[r] = c
	}
	m.PublishRowsReset()
	if m.onChange != nil {
		m.onChange()
	}
}

func (m *filesModel) summary() (n int, size int64) {
	for _, r := range m.rows {
		if m.checked[r] {
			n++
			size += m.files[r].Size
		}
	}
	return
}

// selection returns the 1-based file numbers for aria2's --select-file,
// or "" when everything is selected. Padding files are always included.
func (m *filesModel) selection() string {
	var parts []string
	all := true
	for i, f := range m.files {
		if m.checked[i] || f.Pad {
			parts = append(parts, strconv.Itoa(i+1))
		} else {
			all = false
		}
	}
	if all {
		return ""
	}
	return strings.Join(parts, ",")
}

// askFiles shows the file list and returns the chosen selection.
// It must be called on the UI thread.
func askFiles(owner walk.Form, m *torrentMeta) (string, bool) {
	fm := newFilesModel(m)
	var dlg *walk.Dialog
	var acceptPB, cancelPB *walk.PushButton
	var sumLbl *walk.Label

	text := func() string {
		n, size := fm.summary()
		return fmt.Sprintf("%d of %d files selected (%s)", n, len(fm.rows), humanBytes(float64(size)))
	}
	fm.onChange = func() {
		if sumLbl != nil {
			sumLbl.SetText(text())
		}
	}
	ok := false
	d.Dialog{
		AssignTo: &dlg, Title: "Choose files to download", DefaultButton: &acceptPB, CancelButton: &cancelPB,
		MinSize: d.Size{Width: 640, Height: 440}, Layout: d.VBox{},
		Children: []d.Widget{
			d.Label{Text: m.Name},
			d.TableView{
				CheckBoxes:       true,
				AlternatingRowBG: true,
				Columns:          []d.TableViewColumn{{Title: "File", Width: 470}, {Title: "Size", Width: 100}},
				Model:            fm,
			},
			d.Label{AssignTo: &sumLbl, Text: text()},
			d.Composite{Layout: d.HBox{MarginsZero: true}, Children: []d.Widget{
				d.PushButton{Text: "Select all", OnClicked: func() { fm.setAll(true) }},
				d.PushButton{Text: "Select none", OnClicked: func() { fm.setAll(false) }},
				d.HSpacer{},
				d.PushButton{AssignTo: &acceptPB, Text: "Download", OnClicked: func() {
					if n, _ := fm.summary(); n == 0 {
						walk.MsgBox(dlg, "Sabo DM", "Select at least one file.", walk.MsgBoxIconWarning)
						return
					}
					ok = true
					dlg.Accept()
				}},
				d.PushButton{AssignTo: &cancelPB, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Run(owner)
	if !ok {
		return "", false
	}
	return fm.selection(), true
}
