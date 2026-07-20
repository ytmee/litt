package tui

import (
	"fmt"

	"charm.land/bubbletea/v2"
	"github.com/ytmee/litt/internal/store"
)

func Run(s *store.Store) error {
	m := newModel(s)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	_ = final
	return nil
}
