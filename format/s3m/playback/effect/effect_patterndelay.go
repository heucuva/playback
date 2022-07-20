package effect

import (
	"fmt"

	"github.com/gotracker/playback"
	"github.com/gotracker/playback/format/s3m/layout/channel"
	effectIntf "github.com/gotracker/playback/format/s3m/playback/effect/intf"
)

// PatternDelay defines a pattern delay effect
type PatternDelay ChannelCommand // 'SEx'

// PreStart triggers when the effect enters onto the channel state
func (e PatternDelay) PreStart(cs playback.Channel[channel.Memory, channel.Data], p playback.Playback) error {
	m := p.(effectIntf.S3M)
	return m.SetPatternDelay(int(channel.DataEffect(e) & 0x0F))
}

// Start triggers on the first tick, but before the Tick() function is called
func (e PatternDelay) Start(cs playback.Channel[channel.Memory, channel.Data], p playback.Playback) error {
	cs.ResetRetriggerCount()
	return nil
}

func (e PatternDelay) String() string {
	return fmt.Sprintf("S%0.2x", channel.DataEffect(e))
}
