package period

import (
	"fmt"

	"github.com/heucuva/comparison"
)

type ComponentValue[T any] interface {
	AddInteger(delta int, sign int) T
	AddDelta(delta Delta, sign int) T
	Compare(rhs T) comparison.Spaceship
	GetFrequency(clock Frequency) Frequency
}

type Component[T ComponentValue[T]] struct {
	Value     T
	FreqRatio Frequency
}

// Add adds the current period to a delta value then returns the resulting period.
// This may result in an integer truncation of the underlying value.
func (c Component[T]) AddInteger(delta int, sign int) Component[T] {
	c.Value = c.Value.AddInteger(delta, sign)
	return c
}

// Add adds the current period to a delta value then returns the resulting period.
func (c Component[T]) AddDelta(delta Delta, sign int) Component[T] {
	c.Value = c.Value.AddDelta(delta, sign)
	return c
}

// Compare returns:
//  -1 if the current period is higher frequency than the `rhs` period
//  0 if the current period is equal in frequency to the `rhs` period
//  1 if the current period is lower frequency than the `rhs` period
func (c Component[T]) Compare(rhs Component[T]) comparison.Spaceship {
	return c.Value.Compare(rhs.Value)
}

func (c Component[T]) GetSamplerAdd(baseClock Frequency, samplerSpeed Frequency) float64 {
	return float64(c.GetFrequency(baseClock) / samplerSpeed)
}

func (c Component[T]) GetFrequency(baseClock Frequency) Frequency {
	return c.Value.GetFrequency(baseClock) * c.FreqRatio
}

func (c Component[T]) String() string {
	return fmt.Sprintf("Value:%v Ratio:%v", c.Value, c.FreqRatio)
}
