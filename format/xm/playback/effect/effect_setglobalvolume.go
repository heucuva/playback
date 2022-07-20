package effect

import (
	"fmt"

	xmVolume "github.com/gotracker/playback/format/xm/conversion/volume"
	"github.com/gotracker/playback/format/xm/layout/channel"
	"github.com/gotracker/playback/player/intf"
)

// SetGlobalVolume defines a set global volume effect
type SetGlobalVolume channel.DataEffect // 'G'

// PreStart triggers when the effect enters onto the channel state
func (e SetGlobalVolume) PreStart(cs intf.Channel[channel.Memory, channel.Data], p intf.Playback) error {
	v := xmVolume.XmVolume(e)
	p.SetGlobalVolume(v.Volume())
	return nil
}

// Start triggers on the first tick, but before the Tick() function is called
func (e SetGlobalVolume) Start(cs intf.Channel[channel.Memory, channel.Data], p intf.Playback) error {
	cs.ResetRetriggerCount()
	return nil
}

func (e SetGlobalVolume) String() string {
	return fmt.Sprintf("G%0.2x", channel.DataEffect(e))
}
