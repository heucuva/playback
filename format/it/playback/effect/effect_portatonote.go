package effect

import (
	"fmt"

	"github.com/gotracker/playback"
	"github.com/gotracker/playback/format/it/layout/channel"
	"github.com/gotracker/playback/note"
	"github.com/heucuva/comparison"
)

// PortaToNote defines a portamento-to-note effect
type PortaToNote channel.DataEffect // 'G'

// Start triggers on the first tick, but before the Tick() function is called
func (e PortaToNote) Start(cs playback.Channel[channel.Memory, channel.Data], p playback.Playback) error {
	cs.ResetRetriggerCount()
	cs.UnfreezePlayback()
	if cmd := cs.GetData(); cmd != nil && cmd.HasNote() {
		cs.SetPortaTargetPeriod(cs.GetTargetPeriod())
		cs.SetNotePlayTick(false, note.ActionContinue, 0)
	}
	return nil
}

// Tick is called on every tick
func (e PortaToNote) Tick(cs playback.Channel[channel.Memory, channel.Data], p playback.Playback, currentTick int) error {
	mem := cs.GetMemory()
	xx := mem.PortaToNote(channel.DataEffect(e))

	// vibrato modifies current period for portamento
	period := cs.GetPeriod()
	if period == nil {
		return nil
	}
	period = period.AddDelta(cs.GetPeriodDelta()).(note.Period)
	ptp := cs.GetPortaTargetPeriod()
	if !mem.Shared.OldEffectMode || currentTick != 0 {
		if note.ComparePeriods(period, ptp) == comparison.SpaceshipRightGreater {
			return doPortaUpToNote(cs, float32(xx), 4, ptp, mem.Shared.LinearFreqSlides) // subtracts
		} else {
			return doPortaDownToNote(cs, float32(xx), 4, ptp, mem.Shared.LinearFreqSlides) // adds
		}
	}
	return nil
}

func (e PortaToNote) String() string {
	return fmt.Sprintf("G%0.2x", channel.DataEffect(e))
}
