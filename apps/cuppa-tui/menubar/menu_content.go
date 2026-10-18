package menubar

// Action names what a menu item does; the shell maps them to behaviour.
type Action string

// Every menu action.
const (
	FileNew        Action = "file.new"
	FileOpen       Action = "file.open"
	FileSave       Action = "file.save"
	FileSaveAs     Action = "file.saveas"
	FileQuit       Action = "file.quit"
	EditUndo       Action = "edit.undo"
	EditRedo       Action = "edit.redo"
	EditDuplicate  Action = "edit.duplicate"
	EditCopy       Action = "edit.copy"
	EditPaste      Action = "edit.paste"
	EditDelete     Action = "edit.delete"
	EditGroup      Action = "edit.group"
	EditUngroup    Action = "edit.ungroup"
	EditComponent  Action = "edit.component"
	EditPacks      Action = "edit.packs"
	EditToFront    Action = "edit.tofront"
	EditForward    Action = "edit.forward"
	EditBackward   Action = "edit.backward"
	EditToBack     Action = "edit.toback"
	ViewPreview    Action = "view.preview"
	ViewCursor     Action = "view.cursor"
	ExportPNG      Action = "export.png"
	ExportSVG      Action = "export.svg"
	ExportWebP     Action = "export.webp"
	ExportANSI     Action = "export.ansi"
	ExportText     Action = "export.text"
	HelpShortcuts  Action = "help.shortcuts"
	HelpAbout      Action = "help.about"
	nothing        Action = ""
	separatorLabel        = "-"
)

type item struct {
	label    string
	shortcut string
	action   Action
}

type menu struct {
	label string
	// mnemonic is the letter that opens the menu with Alt held.
	mnemonic rune
	items    []item
}

// menus is the whole menu bar, left to right.
var menus = []menu{
	{"File", 'f', []item{
		{"New", "Ctrl+N", FileNew},
		{"Open…", "Ctrl+O", FileOpen},
		{separatorLabel, "", nothing},
		{"Save", "Ctrl+S", FileSave},
		{"Save As…", "Ctrl+Shift+S", FileSaveAs},
		{separatorLabel, "", nothing},
		{separatorLabel, "", nothing},
		{"Quit", "Ctrl+Q", FileQuit},
	}},
	{"Edit", 'e', []item{
		{"Undo", "Ctrl+Z", EditUndo},
		{"Redo", "Ctrl+Y / Ctrl+Shift+Z", EditRedo},
		{separatorLabel, "", nothing},
		{"Copy", "Ctrl+C", EditCopy},
		{"Paste", "Ctrl+V", EditPaste},
		{"Duplicate", "Ctrl+D", EditDuplicate},
		{"Delete", "Del", EditDelete},
		{separatorLabel, "", nothing},
		{"Group", "Ctrl+G", EditGroup},
		{"Ungroup", "Ctrl+U", EditUngroup},
		{separatorLabel, "", nothing},
		{"To front", "Ctrl+Shift+]", EditToFront},
		{"Forward one", "Ctrl+]", EditForward},
		{"Back one", "Ctrl+[", EditBackward},
		{"To back", "Ctrl+Shift+[", EditToBack},
		{separatorLabel, "", nothing},
		{"Save as component…", "", EditComponent},
		{separatorLabel, "", nothing},
		{"Component packs…", "", EditPacks},
	}},
	{"View", 'v', []item{
		{"Preview design", "Ctrl+P", ViewPreview},
		{"Canvas cursor", "", ViewCursor},
	}},
	{"Export", 'x', []item{
		{"Image (PNG)…", "", ExportPNG},
		{"Image (SVG)…", "", ExportSVG},
		{"Image (WebP)…", "", ExportWebP},
		{separatorLabel, "", nothing},
		{"Colour text (ANSI)…", "", ExportANSI},
		{"Plain text…", "", ExportText},
	}},
	{"Help", 'h', []item{
		{"Shortcuts", "", HelpShortcuts},
		{"About Cuppa", "", HelpAbout},
	}},
}
