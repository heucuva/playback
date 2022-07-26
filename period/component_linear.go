package period

import (
	"github.com/gotracker/playback/note"
	"github.com/heucuva/comparison"
)

type FinetuneLookup interface {
	GetFrequencyForFinetune(clock Frequency, ft note.Finetune) Frequency
}

type LinearPeriod[TFL FinetuneLookup] note.Finetune

func (p LinearPeriod[TFL]) AddInteger(delta, sign int) LinearPeriod[TFL] {
	return p.AddDelta(delta, sign)
}

func (p LinearPeriod[TFL]) AddDelta(delta Delta, sign int) LinearPeriod[TFL] {
	// 0 means "not playing", so keep it that way
	if p <= 0 {
		return 0
	}

	d := ToPeriodDelta(delta) * PeriodDelta(sign)
	p += LinearPeriod[TFL](d)
	if p < 1 {
		p = 1
	}
	return p
}

func (p LinearPeriod[TFL]) Compare(rhs LinearPeriod[TFL]) comparison.Spaceship {
	switch {
	case p < rhs:
		return comparison.SpaceshipRightGreater
	case p > rhs:
		return comparison.SpaceshipLeftGreater
	default:
		return comparison.SpaceshipEqual
	}
}

func (p LinearPeriod[TFL]) GetFrequency(clock Frequency) Frequency {
	var finetuneLookup TFL
	return finetuneLookup.GetFrequencyForFinetune(clock, note.Finetune(p))
}
