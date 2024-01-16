package machine

import (
	"errors"

	"github.com/gotracker/playback/frequency"
	"github.com/gotracker/playback/index"
	"github.com/gotracker/playback/output"
	"github.com/gotracker/playback/player/sampler"
	"github.com/gotracker/playback/song"
)

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) Tick(s *sampler.Sampler) (*output.PremixData, error) {
	if m.opl2Enabled && m.opl2 == nil && m.ms.OPL2Enabled {
		if err := m.setupOPL2(s); err != nil {
			return nil, err
		}
	}

	if err := m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.DoNoteAction(ch, m, frequency.Frequency(s.SampleRate)); err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	premix, err := m.render(s)
	if err != nil {
		return premix, err
	}

	tickErr := runTick(&m.ticker, m)
	if tickErr != nil {
		if !errors.Is(tickErr, song.ErrStopSong) {
			return nil, tickErr
		}
	}

	m.age++
	return premix, errors.Join(tickErr, err)
}

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) onTick() error {
	return m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.Tick(ch, m); err != nil {
			return false, err
		}

		c.updatePastNotes(m)
		return true, nil
	})
}

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) onOrderStart() error {
	return m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.OrderStart(ch, m); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) onRowStart() error {
	rowData, err := m.getRowData()
	if err != nil {
		return err
	}

	m.rowStringer = m.songData.GetRowRenderStringer(rowData, len(m.channels), m.us.LongChannelOutput)

	trace(m, m.rowStringer.String())

	if err := m.singleRowRowStart(); err != nil {
		return err
	}

	if err := m.updateInstructions(rowData); err != nil {
		return err
	}

	return m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.RowStart(ch, m); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) onRowEnd() error {
	return m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.RowEnd(ch, m); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (m *machine[TPeriod, TGlobalVolume, TMixingVolume, TVolume, TPanning]) onOrderEnd() error {
	return m.songData.ForEachChannel(true, func(ch index.Channel) (bool, error) {
		c := &m.channels[ch]
		if err := c.OrderEnd(ch, m); err != nil {
			return false, err
		}
		return true, nil
	})
}
