package period

import (
	"math"

	"github.com/heucuva/comparison"
)

type AmigaPeriod float64

func (p AmigaPeriod) AddInteger(delta int, sign int) AmigaPeriod {
	if p <= 0 {
		return 0
	}
	d := AmigaPeriod(ToPeriodDelta(delta))
	p = AmigaPeriod(math.Trunc(float64(p)))
	p += d * AmigaPeriod(sign)
	return p
}

func (p AmigaPeriod) AddDelta(delta Delta, sign int) AmigaPeriod {
	if p <= 0 {
		return 0
	}
	d := AmigaPeriod(ToPeriodDelta(delta))
	p += d * AmigaPeriod(sign)
	return p
}

func (p AmigaPeriod) Compare(rhs AmigaPeriod) comparison.Spaceship {
	// Amiga periods are lower period == higher frequency and vice versa
	switch {
	case p <= 0 && rhs != 0:
		return comparison.SpaceshipRightGreater
	case p != 0 && rhs <= 0:
		return comparison.SpaceshipLeftGreater
	case p < rhs:
		return comparison.SpaceshipLeftGreater
	case p > rhs:
		return comparison.SpaceshipRightGreater
	default:
		return comparison.SpaceshipEqual
	}
}

func (p AmigaPeriod) GetFrequency(clock Frequency) Frequency {
	if p == 0 {
		return 0
	}
	return clock / Frequency(p)
}
