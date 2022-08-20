package filter

import (
	"math"

	"github.com/gotracker/gomixing/mixing"
	"github.com/gotracker/gomixing/volume"
)

type MovingAverage struct {
	points   mixing.MixBuffer
	coeffs   []volume.Volume
	writePos int
}

func NewMovingAverage(windowSize int) Filter {
	if windowSize == 0 {
		panic("windowSize cannot be 0")
	}
	ma := MovingAverage{
		points: make(mixing.MixBuffer, windowSize),
		coeffs: make([]volume.Volume, windowSize),
	}

	// sigma = how wide we want our bell to be
	sigma := float64(windowSize) / (2.0 * math.Pi)
	// a = our normalizing constant (how tall the bell is)
	a := 1.0 / (sigma * math.Sqrt(2.0*math.Pi))
	// mu = the centerpoint of the bell
	mu := float64(windowSize) / 2.0

	twoSigmaSq := 2 * sigma * sigma
	var acc volume.Volume
	for x := 0; x < windowSize; x++ {
		xmu := (float64(x) + 0.5) - mu
		coeff := volume.Volume(a * math.Exp(-(xmu*xmu)/twoSigmaSq))
		// clamp our value
		if coeff < 0 {
			coeff = 0
		}
		if coeff > 1 {
			coeff = 1
		}
		ma.coeffs[x] = coeff
		acc += coeff
	}
	// Normalize
	if acc != 1.0 && acc != 0.0 {
		v := 1.0 / acc
		for x := 0; x < windowSize; x++ {
			ma.coeffs[x] *= v
		}
	}
	return &ma
}

func (ma MovingAverage) Clone() Filter {
	clone := MovingAverage{
		points:   make(mixing.MixBuffer, len(ma.points)),
		coeffs:   make([]volume.Volume, len(ma.coeffs)),
		writePos: ma.writePos,
	}
	copy(clone.points, ma.points)
	copy(clone.coeffs, ma.coeffs)
	return &clone
}

func (ma *MovingAverage) Filter(dry volume.Matrix) volume.Matrix {
	if dry.Channels == 0 {
		return dry
	}

	wet := volume.Matrix{
		Channels: dry.Channels,
	}

	windowLen := len(ma.points)
	// now set our dry data into the buffer
	ma.points[ma.writePos] = dry
	// add up the points and apply the coefficients
	cpos := ma.writePos
	for _, coeff := range ma.coeffs {
		cpos++
		if cpos >= windowLen {
			cpos -= windowLen
		}
		conv := ma.points[cpos].Apply(coeff)
		wet.Accumulate(conv)
	}

	// shift our history
	ma.writePos++
	if ma.writePos >= windowLen {
		ma.writePos -= windowLen
	}

	return wet
}

func (ma *MovingAverage) UpdateEnv(val int8) {

}
