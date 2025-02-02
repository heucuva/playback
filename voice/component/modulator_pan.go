package component

import (
	"fmt"

	"github.com/gotracker/gomixing/panning"
	"github.com/gotracker/playback/index"
	"github.com/gotracker/playback/tracing"
	"github.com/gotracker/playback/voice/types"
)

// PanModulator is an pan (spatial) modulator
type PanModulator[TPanning types.Panning] struct {
	settings PanModulatorSettings[TPanning]
	unkeyed  struct {
		pan TPanning
	}
	keyed struct {
		delta types.PanDelta
	}
	final panning.Position
}

type PanModulatorSettings[TPanning types.Panning] struct {
	Enabled    bool
	InitialPan TPanning
}

func (p *PanModulator[TPanning]) Setup(settings PanModulatorSettings[TPanning]) {
	p.settings = settings
	p.unkeyed.pan = settings.InitialPan
	p.Reset()
}

func (p *PanModulator[TPanning]) Reset() error {
	p.keyed.delta = 0
	return p.updateFinal()
}

func (p PanModulator[TPanning]) Clone() PanModulator[TPanning] {
	m := p
	return m
}

// SetPan sets the current panning
func (p *PanModulator[TPanning]) SetPan(pan TPanning) error {
	if !p.settings.Enabled {
		return nil
	}

	p.unkeyed.pan = pan
	return p.updateFinal()
}

// GetPan returns the current panning
func (p PanModulator[TPanning]) GetPan() TPanning {
	return p.unkeyed.pan
}

// SetPanDelta sets the current panning delta
func (p *PanModulator[TPanning]) SetPanDelta(d types.PanDelta) error {
	if !p.settings.Enabled {
		return nil
	}

	p.keyed.delta = d
	return p.updateFinal()
}

// GetPanDelta returns the current panning delta
func (p PanModulator[TPanning]) GetPanDelta() types.PanDelta {
	return p.keyed.delta
}

// GetFinalPan returns the current panning
func (p PanModulator[TPanning]) GetFinalPan() panning.Position {
	return p.final
}

func (p PanModulator[TPanning]) DumpState(ch index.Channel, t tracing.Tracer, comment string) {
	t.TraceChannelWithComment(ch, fmt.Sprintf("pan{%v} delta{%v}",
		p.unkeyed.pan,
		p.keyed.delta,
	), comment)
}

func (p *PanModulator[TPanning]) updateFinal() error {
	p.final = types.AddPanningDelta(p.unkeyed.pan, p.keyed.delta).ToPosition()
	return nil
}
