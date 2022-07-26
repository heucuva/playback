package period

import (
	"github.com/gotracker/playback/note"
	"github.com/gotracker/playback/period"
)

const (
	// MiddleCFrequency is the default C2SPD for XM samples
	MiddleCFrequency = 8363
	MiddleCPeriod    = 1712

	// BaseClock is the base clock speed of xm files
	BaseClock period.Frequency = MiddleCFrequency * MiddleCPeriod

	keysPerOctave      = 12
	finetunesPerKey    = 64
	finetunesPerOctave = keysPerOctave * finetunesPerKey
)

var semitonePeriodTable = [...]int{27392, 25856, 24384, 23040, 21696, 20480, 19328, 18240, 17216, 16256, 15360, 14496}

// CalcSemitonePeriod calculates the semitone period for it notes
func CalcSemitonePeriod(semi note.Semitone, ft note.Finetune, instFreq period.Frequency, linearFreqSlides bool) period.Period {
	if semi == note.UnchangedSemitone {
		panic("how?")
	}

	if instFreq == 0 {
		instFreq = period.Frequency(MiddleCFrequency)
	}

	if linearFreqSlides {
		return NewLinear(semi, ft, instFreq)
	}

	return NewAmiga(semi, ft, instFreq)
}

// CalcFinetuneC2Spd calculates a new C2SPD after a finetune adjustment
func CalcFinetuneC2Spd(c2spd period.Frequency, finetune note.Finetune, linearFreqSlides bool) period.Frequency {
	if finetune == 0 {
		return c2spd
	}

	nft := 5*finetunesPerOctave + int(finetune)
	p := CalcSemitonePeriod(note.Semitone(nft/finetunesPerKey), note.Finetune(nft%finetunesPerKey), c2spd, linearFreqSlides)
	return period.Frequency(p.GetFrequency())
}

// FrequencyFromSemitone returns the frequency from the semitone (and c2spd)
func FrequencyFromSemitone(semitone note.Semitone, c2spd period.Frequency, linearFreqSlides bool) float32 {
	period := CalcSemitonePeriod(semitone, 0, c2spd, linearFreqSlides)
	return float32(period.GetFrequency())
}
