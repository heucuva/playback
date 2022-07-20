package effect

import (
	"fmt"

	"github.com/gotracker/playback/format/it/layout/channel"
	"github.com/gotracker/playback/player/intf"
	"github.com/gotracker/playback/song/note"
)

// NewNoteActionNoteOff defines a NewNoteAction: Note Off effect
type NewNoteActionNoteOff channel.DataEffect // 'S75'

// Start triggers on the first tick, but before the Tick() function is called
func (e NewNoteActionNoteOff) Start(cs intf.Channel[channel.Memory, channel.Data], p intf.Playback) error {
	cs.SetNewNoteAction(note.ActionRelease)
	return nil
}

func (e NewNoteActionNoteOff) String() string {
	return fmt.Sprintf("S%0.2x", channel.DataEffect(e))
}
