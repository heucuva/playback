package period

import (
	"math"

	"github.com/gotracker/playback/note"
	"github.com/heucuva/comparison"

	"github.com/gotracker/playback/period"
)

// Amiga defines a sampler period that follows the Amiga-style approach of note
// definition. Useful in calculating resampling.
type Amiga struct {
	period.Component[period.AmigaPeriod]
}

func NewAmiga(st note.Semitone, ft note.Finetune, instFreq period.Frequency) Amiga {
	kp := semitonePeriodTable[st.Key()]
	kp >>= int(st.Octave())

	if ft != 0 {
		linFreq := period.Frequency(math.Pow(2, float64(ft)/finetunesPerOctave))
		instFreq /= linFreq
	}

	var p Amiga
	p.Value = period.AmigaPeriod(kp)
	p.FreqRatio = instFreq / MiddleCFrequency
	return p
}

func (c Amiga) AddInteger(delta, sign int) period.Period {
	c.Component = c.Component.AddInteger(delta, sign)
	return c
}

func (c Amiga) AddDelta(delta period.Delta, sign int) period.Period {
	c.Component = c.Component.AddDelta(delta, sign)
	return c
}

func (c Amiga) Compare(rhs period.Period) comparison.Spaceship {
	rp, ok := rhs.(Amiga)
	if !ok {
		panic("cannot compare periods of different kinds")
	}

	return c.Component.Compare(rp.Component)
}

func (c Amiga) GetSamplerAdd(samplerSpeed period.Frequency) float64 {
	return c.Component.GetSamplerAdd(BaseClock, samplerSpeed)
}

func (c Amiga) GetFrequency() period.Frequency {
	return c.Component.GetFrequency(BaseClock)
}
