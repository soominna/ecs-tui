package ui

import "github.com/charmbracelet/bubbles/viewport"

// viewportHelper manages viewport initialization and resizing.
type viewportHelper struct {
	viewport viewport.Model
	ready    bool
}

// handleResize initializes the viewport on first call, or updates its dimensions.
func (h *viewportHelper) handleResize(width, height int) {
	if !h.ready {
		h.viewport = viewport.New(width, height)
		h.ready = true
	} else {
		h.viewport.Width = width
		h.viewport.Height = height
	}
}
