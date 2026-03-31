package layout

const (
	SidebarWidth = 32
	ChatWidth    = 36
	DrawerHeight = 9
	HeaderLines  = 1
	StatusLines  = 1
)

// Zones holds computed dimensions for every region of the TUI.
type Zones struct {
	TotalWidth   int
	TotalHeight  int
	SidebarWidth int
	MainWidth    int
	MainHeight   int
	ChatWidth    int
	ChatOpen     bool
	DrawerHeight int
	DrawerOpen   bool
}

// Compute derives all zone sizes from the terminal dimensions and panel toggles.
func Compute(w, h int, chatOpen, drawerOpen bool) Zones {
	overhead := HeaderLines + StatusLines
	mainH := h - overhead
	if drawerOpen && mainH > DrawerHeight+2 {
		mainH -= DrawerHeight
	}

	mainW := w - SidebarWidth
	chatW := 0
	if chatOpen && mainW > ChatWidth+20 {
		chatW = ChatWidth
		mainW -= chatW
	}

	return Zones{
		TotalWidth:   w,
		TotalHeight:  h,
		SidebarWidth: SidebarWidth,
		MainWidth:    mainW,
		MainHeight:   mainH,
		ChatWidth:    chatW,
		ChatOpen:     chatOpen && chatW > 0,
		DrawerHeight: DrawerHeight,
		DrawerOpen:   drawerOpen,
	}
}
