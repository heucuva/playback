package period

import (
	"math"

	"github.com/gotracker/playback/note"
	"github.com/heucuva/comparison"

	"github.com/gotracker/playback/period"
)

// Linear is a linear period, based on semitone and finetune values
type Linear struct {
	period.Component[period.LinearPeriod[finetuneLookup]]
}

func NewLinear(st note.Semitone, ft note.Finetune, instFreq period.Frequency) Linear {
	finetunes := note.Finetune(st)*finetunesPerKey + ft
	var p Linear
	p.Value = period.LinearPeriod[finetuneLookup](finetunes)
	p.FreqRatio = instFreq / MiddleCFrequency
	return p
}

func (c Linear) AddInteger(delta, sign int) period.Period {
	return c.AddDelta(delta, sign)
}

func (c Linear) AddDelta(delta period.Delta, sign int) period.Period {
	c.Component = c.Component.AddDelta(delta, sign)
	return c
}

func (c Linear) Compare(rhs period.Period) comparison.Spaceship {
	rp, ok := rhs.(Linear)
	if !ok {
		panic("cannot compare periods of different kinds")
	}

	return c.Component.Compare(rp.Component)
}

func (c Linear) GetSamplerAdd(samplerSpeed period.Frequency) float64 {
	return c.Component.GetSamplerAdd(BaseClock, samplerSpeed)
}

func (c Linear) GetFrequency() period.Frequency {
	return c.Component.GetFrequency(BaseClock)
}

type finetuneLookup struct{}

func (finetuneLookup) GetFrequencyForFinetune(clock period.Frequency, ft note.Finetune) period.Frequency {
	linFreq := math.Pow(2, float64(ft)/finetunesPerOctave)

	p := period.Frequency(semitonePeriodTable[0])

	return clock * period.Frequency(linFreq) / p
}
