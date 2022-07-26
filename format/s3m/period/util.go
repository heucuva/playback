package period

import (
	s3mfile "github.com/gotracker/goaudiofile/music/tracked/s3m"
	"github.com/gotracker/playback/note"
	"github.com/gotracker/playback/period"
)

const (
	// MiddleCFrequency is the default C2SPD for S3M samples
	MiddleCFrequency = period.Frequency(s3mfile.DefaultC2Spd)
	MiddleCPeriod    = 1712

	// BaseClock is the base clock speed of S3M files
	BaseClock period.Frequency = MiddleCFrequency * MiddleCPeriod

	keysPerOctave      = 12
	finetunesPerKey    = 64
	finetunesPerOctave = keysPerOctave * finetunesPerKey
)

var semitonePeriodTable = [...]int{27392, 25856, 24384, 23040, 21696, 20480, 19328, 18240, 17216, 16256, 15360, 14496}

// CalcSemitonePeriod calculates the semitone period for it notes
func CalcSemitonePeriod(semi note.Semitone, ft note.Finetune, instFreq period.Frequency) period.Period {
	if semi == note.UnchangedSemitone {
		panic("how?")
	}

	if instFreq == 0 {
		instFreq = period.Frequency(MiddleCFrequency)
	}

	return NewAmiga(semi, ft, instFreq)
}

// CalcFinetuneC2Spd calculates a new C2SPD after a finetune adjustment
func CalcFinetuneC2Spd(c2spd period.Frequency, finetune note.Finetune) period.Frequency {
	if finetune == 0 {
		return c2spd
	}

	nft := 5*finetunesPerOctave + int(finetune)
	p := CalcSemitonePeriod(note.Semitone(nft/finetunesPerKey), note.Finetune(nft%finetunesPerKey), c2spd)
	return period.Frequency(p.GetFrequency())
}

// FrequencyFromSemitone returns the frequency from the semitone (and c2spd)
func FrequencyFromSemitone(semitone note.Semitone, c2spd period.Frequency) float32 {
	period := CalcSemitonePeriod(semitone, 0, c2spd)
	return float32(period.GetFrequency())
}
