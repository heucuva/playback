package filter

import (
	"math"

	"github.com/gotracker/gomixing/volume"

	"github.com/gotracker/playback/frequency"
	"github.com/heucuva/optional"
)

type itResonantFilterChannelData struct {
	ynz1 volume.Volume
	ynz2 volume.Volume
}

// ResonantFilter is a modified 2-pole resonant filter
type ResonantFilter struct {
	channels []itResonantFilterChannelData
	a0       volume.Volume
	b0       volume.Volume
	b1       volume.Volume

	enabled             bool
	resonance           optional.Value[uint8]
	cutoff              optional.Value[uint8]
	highpass            bool
	extendedFilterRange bool

	f2           float64
	fr           float64
	efr          float64
	playbackRate frequency.Frequency
}

type ITResonantFilterParams struct {
	Cutoff              uint8
	Resonance           uint8
	ExtendedFilterRange bool
	Highpass            bool
}

// NewITResonantFilter creates a new resonant filter with the provided cutoff and resonance values
func NewITResonantFilter(cutoff uint8, resonance uint8, extendedFilterRange bool, highpass bool) Filter {
	rf := ResonantFilter{
		highpass:            highpass,
		extendedFilterRange: extendedFilterRange,
	}

	if resonance&0x80 != 0 {
		rf.resonance.Set(uint8(resonance) & 0x7f)
	}
	c := uint8(0x7F)
	if (cutoff & 0x80) != 0 {
		c = cutoff & 0x7f
		rf.cutoff.Set(uint8(c))
	}

	return &rf
}

func (f *ResonantFilter) SetPlaybackRate(playback frequency.Frequency) {
	if f.playbackRate == playback {
		return
	}
	f.playbackRate = playback

	f.f2 = float64(playback) / 2.0

	f.fr = float64(playback)
	if f.fr != 0 {
		f.efr = float64(1) / f.fr
	}

	c := uint8(0x7F)
	if v, set := f.cutoff.Get(); set {
		c = v
	}
	f.recalculate(c)
}

func (f *ResonantFilter) Clone() Filter {
	c := *f
	c.channels = make([]itResonantFilterChannelData, len(f.channels))
	for i := range f.channels {
		c.channels[i] = f.channels[i]
	}
	return &c
}

// Filter processes incoming (dry) samples and produces an outgoing filtered (wet) result
func (f *ResonantFilter) Filter(dry volume.Matrix) volume.Matrix {
	if dry.Channels == 0 {
		return volume.Matrix{}
	}
	wet := dry // we can update in-situ and be ok
	for i := 0; i < dry.Channels; i++ {
		s := dry.StaticMatrix[i]
		for len(f.channels) <= i {
			f.channels = append(f.channels, itResonantFilterChannelData{})
		}
		c := &f.channels[i]

		yn := s
		if f.enabled {
			yn *= f.a0
			yn += c.ynz1*f.b0 + c.ynz2*f.b1
			if yn < -1 {
				yn = -1
			}
			if yn > 1 {
				yn = 1
			}
		}
		c.ynz2 = c.ynz1
		c.ynz1 = yn
		if f.highpass {
			c.ynz1 -= s
		}
		wet.StaticMatrix[i] = yn
	}
	return wet
}

func (f *ResonantFilter) recalculate(v uint8) {
	cutoff, useCutoff := f.cutoff.Get()
	resonance, useResonance := f.resonance.Get()

	if !useResonance {
		resonance = 0
	}

	if !useCutoff {
		cutoff = 127
	} else {
		cutoff = v
		if cutoff > 127 {
			cutoff = 127
		}

		f.cutoff.Set(cutoff)
	}

	computedCutoff := cutoff * 2

	useFilter := true
	if computedCutoff >= 254 && resonance == 0 {
		useFilter = false
	}

	f.enabled = useFilter
	if !f.enabled {
		return
	}

	const (
		itFilterRange  = 24.0 // standard IT range
		extfilterRange = 20.0 // extended OpenMPT range
	)

	filterRange := itFilterRange
	if f.extendedFilterRange {
		filterRange = extfilterRange
	}

	const dampingFactorDivisor = ((24.0 / 128.0) / 20.0)
	dampingFactor := math.Pow(10.0, -float64(resonance)*dampingFactorDivisor)

	freq := f.f2
	fcComputedCutoff := float64(computedCutoff)
	freq = 110.0 * math.Pow(2.0, 0.25+(fcComputedCutoff/filterRange))
	if freq < 120.0 {
		freq = 120.0
	} else if freq > 20000 {
		freq = 20000
	}
	if freq > f.f2 && f.f2 >= 120.0 {
		freq = f.f2
	}

	fc := freq * 4.0 * math.Pi

	var d, e float64
	if f.extendedFilterRange {
		r := fc * f.efr

		d = (1.0 - 2.0*dampingFactor) * r
		if d > 2.0 {
			d = 2.0
		}
		if r != 0 {
			d = (2.0*dampingFactor - d) / r
			e = 1.0 / (r * r)
		} else {
			d = 0
			e = 0
		}
	} else {
		r := f.fr / fc

		d = dampingFactor*r + dampingFactor - 1.0
		e = r * r
	}

	a := 1.0 / (1.0 + d + e)
	b := (d + e + e) * a
	c := -e * a
	if f.highpass {
		a = 1.0 - a
	} else {
		// lowpass
		if a == 0 {
			// prevent silence at extremely low cutoff and very high sampling rate
			a = 1.0
		}
	}

	if math.IsNaN(a) {
		panic("a")
	}
	if math.IsNaN(b) {
		panic("b")
	}
	if math.IsNaN(c) {
		panic("c")
	}

	f.a0 = volume.Volume(a)
	f.b0 = volume.Volume(b)
	f.b1 = volume.Volume(c)
}

// UpdateEnv updates the filter with the value from the filter envelope
func (f *ResonantFilter) UpdateEnv(cutoff uint8) {
	f.recalculate(cutoff)
}
