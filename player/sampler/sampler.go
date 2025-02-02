package sampler

import (
	"github.com/gotracker/gomixing/mixing"
	"github.com/gotracker/playback/frequency"
	"github.com/gotracker/playback/output"
)

// Sampler is a container of sampler/mixer settings
type Sampler struct {
	SampleRate    int
	BaseClockRate frequency.Frequency
	OnGenerate    func(premix *output.PremixData)

	mixer mixing.Mixer
}

// NewSampler returns a new sampler object based on the input settings
func NewSampler(samplesPerSec, channels int, onGenerate func(premix *output.PremixData)) *Sampler {
	s := Sampler{
		SampleRate: samplesPerSec,
		OnGenerate: onGenerate,
		mixer: mixing.Mixer{
			Channels: channels,
		},
	}
	return &s
}

// Mixer returns a pointer to the current mixer object
func (s *Sampler) Mixer() *mixing.Mixer {
	return &s.mixer
}

// GetPanMixer returns the panning mixer that can generate a matrix
// based on input pan value
func (s *Sampler) GetPanMixer() mixing.PanMixer {
	return mixing.GetPanMixer(s.mixer.Channels)
}
