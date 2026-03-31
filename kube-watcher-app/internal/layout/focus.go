package layout

// FocusTarget names the UI region that receives keyboard events.
type FocusTarget int

const (
	FocusSidebar FocusTarget = iota
	FocusMain
	FocusChat
	FocusDrawer
	FocusModal
)
