package load

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	itfile "github.com/gotracker/goaudiofile/music/tracked/it"
	"github.com/gotracker/gomixing/volume"
	"github.com/gotracker/playback/frequency"
	"github.com/gotracker/playback/period"
	"github.com/gotracker/playback/player/feature"
	"github.com/gotracker/playback/util"
	"github.com/gotracker/playback/voice/autovibrato"
	"github.com/gotracker/playback/voice/envelope"
	"github.com/gotracker/playback/voice/fadeout"
	"github.com/gotracker/playback/voice/loop"
	"github.com/gotracker/playback/voice/pcm"
	"github.com/gotracker/playback/voice/pitchpan"
	"github.com/gotracker/playback/voice/types"
	"github.com/heucuva/optional"

	"github.com/gotracker/playback/filter"
	itNote "github.com/gotracker/playback/format/it/note"
	itPanning "github.com/gotracker/playback/format/it/panning"
	itVolume "github.com/gotracker/playback/format/it/volume"
	"github.com/gotracker/playback/instrument"
	"github.com/gotracker/playback/note"
	oscillatorImpl "github.com/gotracker/playback/oscillator"
)

type convInst[TPeriod period.Period] struct {
	Inst *instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]
	NR   []noteRemap
}

type convertITInstrumentSettings struct {
	linearFrequencySlides bool
	extendedFilterRange   bool
	useHighPassFilter     bool
}

func convertITInstrumentOldToInstrument[TPeriod period.Period](inst *itfile.IMPIInstrumentOld, pc period.PeriodConverter[TPeriod], sampData []itfile.FullSample, convSettings convertITInstrumentSettings, features []feature.Feature) (map[int]*convInst[TPeriod], error) {
	outInsts := make(map[int]*convInst[TPeriod])

	if err := buildNoteSampleKeyboard(outInsts, inst.NoteSampleKeyboard[:]); err != nil {
		return nil, err
	}

	for i, ci := range outInsts {
		volEnvLoopMode := loop.ModeDisabled
		volEnvLoopSettings := loop.Settings{
			Begin: int(inst.VolumeLoopStart),
			End:   int(inst.VolumeLoopEnd),
		}
		volEnvSustainMode := loop.ModeDisabled
		volEnvSustainSettings := loop.Settings{
			Begin: int(inst.SustainLoopStart),
			End:   int(inst.SustainLoopEnd),
		}

		id := instrument.PCM[itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
			FadeOut: fadeout.Settings{
				Mode:   fadeout.ModeAlwaysActive,
				Amount: volume.Volume(inst.Fadeout) / 512,
			},
			VolEnv: envelope.Envelope[itVolume.Volume]{
				Enabled: (inst.Flags & itfile.IMPIOldFlagUseVolumeEnvelope) != 0,
				Values:  make([]envelope.Point[itVolume.Volume], 0),
			},
		}

		ii := instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
			Inst: &id,
			Static: instrument.StaticValues[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
				PC: pc,
			},
		}

		switch inst.NewNoteAction {
		case itfile.NewNoteActionCut:
			ii.Static.NewNoteAction = note.ActionCut
		case itfile.NewNoteActionContinue:
			ii.Static.NewNoteAction = note.ActionContinue
		case itfile.NewNoteActionOff:
			ii.Static.NewNoteAction = note.ActionRelease
		case itfile.NewNoteActionFade:
			ii.Static.NewNoteAction = note.ActionFadeout
		default:
			ii.Static.NewNoteAction = note.ActionCut
		}

		ci.Inst = &ii
		if err := addSampleInfoToConvertedInstrument(ci.Inst, &id, &sampData[i], volume.Volume(1), convSettings, features); err != nil {
			return nil, err
		}

		if id.VolEnv.Enabled && id.VolEnv.Loop.Length() >= 0 {
			if enabled := (inst.Flags & itfile.IMPIOldFlagUseVolumeLoop) != 0; enabled {
				volEnvLoopMode = loop.ModeNormal
			}
			if enabled := (inst.Flags & itfile.IMPIOldFlagUseSustainVolumeLoop) != 0; enabled {
				volEnvSustainMode = loop.ModeNormal
			}

			for i := range inst.VolumeEnvelope {
				var out envelope.Point[itVolume.Volume]
				in1 := inst.VolumeEnvelope[i]
				vol := itVolume.Volume(uint8(in1))
				if vol > itVolume.Volume(itVolume.MaxItVolume) {
					vol = itVolume.Volume(itVolume.MaxItVolume)
				}
				out.Pos = i
				out.Y = vol
				ending := false
				if i+1 >= len(inst.VolumeEnvelope) {
					ending = true
				} else {
					in2 := inst.VolumeEnvelope[i+1]
					if in2 == 0xFF {
						ending = true
					}
				}
				if !ending {
					out.Length = 1
				} else {
					id.VolEnv.Length = i
					out.Length = math.MaxInt64
				}
				id.VolEnv.Values = append(id.VolEnv.Values, out)
			}

			id.VolEnv.Loop = loop.NewLoop(volEnvLoopMode, volEnvLoopSettings)
			id.VolEnv.Sustain = loop.NewLoop(volEnvSustainMode, volEnvSustainSettings)
		}
	}

	return outInsts, nil
}

func convertITInstrumentToInstrument[TPeriod period.Period](inst *itfile.IMPIInstrument, pc period.PeriodConverter[TPeriod], sampData []itfile.FullSample, convSettings convertITInstrumentSettings, pluginFilters map[int]filter.Info, features []feature.Feature) (map[int]*convInst[TPeriod], error) {
	outInsts := make(map[int]*convInst[TPeriod])

	if err := buildNoteSampleKeyboard(outInsts, inst.NoteSampleKeyboard[:]); err != nil {
		return nil, err
	}

	var (
		voiceFilter  filter.Info
		pluginFilter filter.Info
	)
	if inst.InitialFilterResonance != 0 {
		voiceFilter.Name = "itresonant"
		voiceFilter.Params = filter.ITResonantFilterParams{
			Cutoff:              inst.InitialFilterCutoff,
			Resonance:           inst.InitialFilterResonance,
			ExtendedFilterRange: convSettings.extendedFilterRange,
			Highpass:            convSettings.useHighPassFilter,
		}
	}

	if inst.MidiChannel >= 0x81 {
		if pf, ok := pluginFilters[int(inst.MidiChannel)-0x81]; ok {
			pluginFilter = pf
		}
	}

	for i, ci := range outInsts {
		id := instrument.PCM[itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
			FadeOut: fadeout.Settings{
				Mode:   fadeout.ModeAlwaysActive,
				Amount: volume.Volume(inst.Fadeout) / 1024,
			},
			PitchPan: pitchpan.PitchPan{
				Enabled:    inst.PitchPanSeparation != 0,
				Center:     note.Semitone(inst.PitchPanCenter),
				Separation: float32(inst.PitchPanSeparation) / 8,
			},
		}

		ii := instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
			Static: instrument.StaticValues[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]{
				PC:           pc,
				VoiceFilter:  voiceFilter,
				PluginFilter: pluginFilter,
			},
			Inst: &id,
		}

		switch inst.NewNoteAction {
		case itfile.NewNoteActionCut:
			ii.Static.NewNoteAction = note.ActionCut
		case itfile.NewNoteActionContinue:
			ii.Static.NewNoteAction = note.ActionContinue
		case itfile.NewNoteActionOff:
			ii.Static.NewNoteAction = note.ActionRelease
		case itfile.NewNoteActionFade:
			ii.Static.NewNoteAction = note.ActionFadeout
		default:
			ii.Static.NewNoteAction = note.ActionCut
		}

		mixVol := volume.Volume(inst.GlobalVolume.Value())
		if !inst.DefaultPan.IsDisabled() {
			ii.Static.Panning = optional.NewValue(util.Lerp(float64(inst.DefaultPan.Value()), 0, itPanning.MaxPanning))
		}

		ci.Inst = &ii
		if err := addSampleInfoToConvertedInstrument(ci.Inst, &id, &sampData[i], mixVol, convSettings, features); err != nil {
			return nil, err
		}

		if err := convertEnvelope(&id.VolEnv, &inst.VolumeEnvelope, convertVolEnvValue); err != nil {
			return nil, err
		}
		id.VolEnvFinishFadesOut = true

		if err := convertEnvelope(&id.PanEnv, &inst.PanningEnvelope, convertPanEnvValue); err != nil {
			return nil, err
		}

		id.PitchFiltMode = (inst.PitchEnvelope.Flags & 0x80) != 0 // special flag (IT format changes pitch to resonant filter cutoff envelope)
		if err := convertEnvelope(&id.PitchFiltEnv, &inst.PitchEnvelope, convertPitchEnvValue); err != nil {
			return nil, err
		}
	}

	return outInsts, nil
}

func convertVolEnvValue(v int8) itVolume.Volume {
	vol := itVolume.Volume(uint8(v))
	if vol > itVolume.Volume(itVolume.MaxItVolume) {
		// NOTE: there might be an incoming Y value == 0xFF, which really
		// means "end of envelope" and should not mean "full volume",
		// but we can cheat a little here and probably get away with it...
		vol = itVolume.Volume(itVolume.MaxItVolume)
	}
	return vol
}

func convertPanEnvValue(v int8) itPanning.Panning {
	return itPanning.Panning(int(v) + 128)
}

func convertPitchEnvValue(v int8) types.PitchFiltValue {
	return types.PitchFiltValue(v)
}

func convertEnvelope[T any](outEnv *envelope.Envelope[T], inEnv *itfile.Envelope, convert func(int8) T) error {
	outEnv.Enabled = (inEnv.Flags & itfile.EnvelopeFlagEnvelopeOn) != 0
	if !outEnv.Enabled {
		var disabled loop.Disabled
		outEnv.Loop = &disabled
		outEnv.Sustain = &disabled
		return nil
	}

	envLoopMode := loop.ModeDisabled
	envLoopSettings := loop.Settings{
		Begin: int(inEnv.LoopBegin),
		End:   int(inEnv.LoopEnd),
	}
	if enabled := (inEnv.Flags & itfile.EnvelopeFlagLoopOn) != 0; enabled {
		envLoopMode = loop.ModeNormal
	}
	envSustainMode := loop.ModeDisabled
	envSustainSettings := loop.Settings{
		Begin: int(inEnv.SustainLoopBegin),
		End:   int(inEnv.SustainLoopEnd),
	}
	if enabled := (inEnv.Flags & itfile.EnvelopeFlagSustainLoopOn) != 0; enabled {
		envSustainMode = loop.ModeNormal
	}
	outEnv.Values = make([]envelope.Point[T], int(inEnv.Count))
	for i := range outEnv.Values {
		in1 := inEnv.NodePoints[i]
		y := convert(in1.Y)
		if i+1 < len(outEnv.Values) {
			in2 := inEnv.NodePoints[i+1]
			ticks := int(in2.Tick) - int(in1.Tick)
			var out envelope.Point[T]
			out.Length = ticks
			out.Pos = int(in1.Tick)
			out.Y = y
			outEnv.Values[i] = out
		} else {
			outEnv.Values[i] = envelope.Point[T]{
				Pos:    int(in1.Tick),
				Length: math.MaxInt,
				Y:      y,
			}
			outEnv.Length = int(in1.Tick)
		}
	}

	outEnv.Loop = loop.NewLoop(envLoopMode, envLoopSettings)
	outEnv.Sustain = loop.NewLoop(envSustainMode, envSustainSettings)

	return nil
}

func buildNoteSampleKeyboard[TPeriod period.Period](noteKeyboard map[int]*convInst[TPeriod], nsk []itfile.NoteSample) error {
	for o, ns := range nsk {
		s := int(ns.Sample)
		if s == 0 {
			continue
		}
		si := int(ns.Sample) - 1
		if si < 0 {
			continue
		}
		n := itNote.FromItNote(ns.Note)
		if nn, ok := n.(note.Normal); ok {
			st := note.Semitone(nn)
			ci, ok := noteKeyboard[si]
			if !ok {
				ci = &convInst[TPeriod]{}
				noteKeyboard[si] = ci
			}
			ci.NR = append(ci.NR, noteRemap{
				Orig:  note.Semitone(o),
				Remap: st,
			})
		}
	}

	return nil
}

func getSampleFormat(is16Bit bool, isSigned bool, isBigEndian bool) pcm.SampleDataFormat {
	if is16Bit {
		if isSigned {
			if isBigEndian {
				return pcm.SampleDataFormat16BitBESigned
			}
			return pcm.SampleDataFormat16BitLESigned
		} else if isBigEndian {
			return pcm.SampleDataFormat16BitLEUnsigned
		}
		return pcm.SampleDataFormat16BitLEUnsigned
	} else if isSigned {
		return pcm.SampleDataFormat8BitSigned
	}
	return pcm.SampleDataFormat8BitUnsigned
}

func itAutoVibratoWSToProtrackerWS(vibtype uint8) uint8 {
	switch vibtype {
	case 0:
		return uint8(oscillatorImpl.WaveTableSelectSineRetrigger)
	case 1:
		return uint8(oscillatorImpl.WaveTableSelectSawtoothRetrigger)
	case 2:
		return uint8(oscillatorImpl.WaveTableSelectSquareRetrigger)
	case 3:
		return uint8(oscillatorImpl.WaveTableSelectRandomRetrigger)
	case 4:
		return uint8(oscillatorImpl.WaveTableSelectInverseSawtoothRetrigger)
	default:
		return uint8(oscillatorImpl.WaveTableSelectSineRetrigger)
	}
}

func addSampleInfoToConvertedInstrument[TPeriod period.Period](ii *instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning], id *instrument.PCM[itVolume.FineVolume, itVolume.Volume, itPanning.Panning], si *itfile.FullSample, instVol volume.Volume, convSettings convertITInstrumentSettings, features []feature.Feature) error {
	instLen := int(si.Header.Length)
	numChannels := 1

	id.MixingVolume.Set(max(itVolume.FineVolume(float32(itVolume.MaxItFineVolume)*si.Header.GlobalVolume.Value()*float32(instVol)), itVolume.MaxItFineVolume))
	loopMode := loop.ModeDisabled
	loopSettings := loop.Settings{
		Begin: int(si.Header.LoopBegin),
		End:   int(si.Header.LoopEnd),
	}
	sustainMode := loop.ModeDisabled
	sustainSettings := loop.Settings{
		Begin: int(si.Header.SustainLoopBegin),
		End:   int(si.Header.SustainLoopEnd),
	}

	if si.Header.Flags.IsLoopEnabled() {
		if si.Header.Flags.IsLoopPingPong() {
			loopMode = loop.ModePingPong
		} else {
			loopMode = loop.ModeNormal
		}
	}

	if si.Header.Flags.IsSustainLoopEnabled() {
		if si.Header.Flags.IsSustainLoopPingPong() {
			sustainMode = loop.ModePingPong
		} else {
			sustainMode = loop.ModeNormal
		}
	}

	id.Loop = loop.NewLoop(loopMode, loopSettings)
	id.SustainLoop = loop.NewLoop(sustainMode, sustainSettings)

	if si.Header.Flags.IsStereo() {
		numChannels = 2
	}

	is16Bit := si.Header.Flags.Is16Bit()
	isSigned := si.Header.ConvertFlags.IsSignedSamples()
	isBigEndian := si.Header.ConvertFlags.IsBigEndian()
	format := getSampleFormat(is16Bit, isSigned, isBigEndian)

	isDeltaSamples := si.Header.ConvertFlags.IsSampleDelta()
	var data []byte
	if si.Header.Flags.IsCompressed() {
		if is16Bit {
			data = uncompress16IT214(si.Data, isBigEndian)
		} else {
			data = uncompress8IT214(si.Data)
		}
		isDeltaSamples = true
	} else {
		data = si.Data
	}

	if isDeltaSamples {
		deltaDecode(data, format)
	}

	bytesPerFrame := numChannels

	if is16Bit {
		bytesPerFrame *= 2
	}

	if len(data) < int(si.Header.Length+1)*bytesPerFrame {
		var value any
		var order binary.ByteOrder = binary.LittleEndian
		if is16Bit {
			if isSigned {
				value = int16(0)
			} else {
				value = uint16(0x8000)
			}
			if isBigEndian {
				order = binary.BigEndian
			}
		} else {
			if isSigned {
				value = int8(0)
			} else {
				value = uint8(0x80)
			}
		}

		buf := bytes.NewBuffer(data)
		for buf.Len() < int(si.Header.Length+1)*bytesPerFrame {
			if err := binary.Write(buf, order, value); err != nil {
				return err
			}
		}
		data = buf.Bytes()
	}

	samp, err := instrument.NewSample(data, instLen, numChannels, format, features)
	if err != nil {
		return err
	}
	id.Sample = samp

	ii.Static.Filename = si.Header.GetFilename()
	ii.Static.Name = si.Header.GetName()
	ii.SampleRate = frequency.Frequency(si.Header.C5Speed)
	ii.Static.AutoVibrato = autovibrato.AutoVibratoConfig[TPeriod]{
		Enabled:           (si.Header.VibratoDepth != 0 && si.Header.VibratoSpeed != 0 && si.Header.VibratoSweep != 0),
		Sweep:             255,
		WaveformSelection: itAutoVibratoWSToProtrackerWS(si.Header.VibratoType),
		Depth:             float32(si.Header.VibratoDepth),
		Rate:              int(si.Header.VibratoSpeed),
		FactoryName:       "autovibrato",
	}
	ii.Static.Volume = itVolume.Volume(si.Header.Volume)

	if ii.SampleRate == 0 {
		ii.SampleRate = 8363.0
	}

	if si.Header.Flags.IsStereo() {
		ii.SampleRate /= 2.0
	}

	if !convSettings.linearFrequencySlides {
		ii.Static.AutoVibrato.Depth /= 64.0
	}

	if si.Header.VibratoSweep != 0 {
		ii.Static.AutoVibrato.Sweep = int(si.Header.VibratoDepth) * 256 / int(si.Header.VibratoSweep)
	}
	if !si.Header.DefaultPan.IsDisabled() {
		id.Panning.Set(itPanning.Panning(si.Header.DefaultPan))
	}

	return nil
}

func itReadbits(n int8, r io.ByteReader, bitnum *uint32, bitbuf *uint32) (uint32, error) {
	var value uint32 = 0
	var i uint32 = uint32(n)

	// this could be better
	for i > 0 {
		i--
		if *bitnum == 0 {
			b, err := r.ReadByte()
			if err != nil {
				return value >> (32 - n), err
			}
			*bitbuf = uint32(b)
			*bitnum = 8
		}
		value >>= 1
		value |= (*bitbuf) << 31
		(*bitbuf) >>= 1
		(*bitnum)--
	}
	return value >> (32 - n), nil
}

// 8-bit sample uncompressor for IT 2.14+
func uncompress8IT214(data []byte) []byte {
	in := bytes.NewReader(data)
	out := &bytes.Buffer{}

	var (
		blklen uint16 // length of compressed data block in samples
		blkpos uint16 // position in block
		width  uint8  // actual "bit width"
		value  uint16 // value read from file to be processed
		v      int8   // sample value

		// state for itReadbits
		bitbuf uint32
		bitnum uint32
	)

	// now unpack data till the dest buffer is full
	for in.Len() > 0 {
		// read a new block of compressed data and reset variables
		// block layout: word size, <size> bytes data
		bitbuf = 0
		bitnum = 0

		blklen = uint16(math.Min(0x8000, float64(in.Len())))
		blkpos = 0

		width = 9 // start with width of 9 bits

		var clen uint16
		if err := binary.Read(in, binary.LittleEndian, &clen); err != nil {
			panic(err)
		}

		// now uncompress the data block
	blockLoop:
		for blkpos < blklen {
			if width > 9 {
				// illegal width, abort
				panic(fmt.Sprintf("Illegal bit width %d for 8-bit sample\n", width))
			}
			vv, err := itReadbits(int8(width), in, &bitnum, &bitbuf)
			if err != nil {
				break blockLoop
			}
			value = uint16(vv)

			if width < 7 {
				// method 1 (1-6 bits)
				// check for "100..."
				if value == 1<<(width-1) {
					// yes!
					vv, err := itReadbits(3, in, &bitnum, &bitbuf) // read new width
					if err != nil {
						break blockLoop
					}
					value = uint16(vv + 1)
					if value < uint16(width) {
						width = uint8(value)
					} else {
						width = uint8(value + 1)
					}
					continue blockLoop // ... next value
				}
			} else if width < 9 {
				// method 2 (7-8 bits)
				var border uint8 = (0xFF >> (9 - width)) - 4 // lower border for width chg
				if value > uint16(border) && value <= (uint16(border)+8) {
					value -= uint16(border) // convert width to 1-8
					if value < uint16(width) {
						width = uint8(value)
					} else {
						width = uint8(value + 1)
					}
					continue blockLoop // ... next value
				}
			} else {
				// method 3 (9 bits)
				// bit 8 set?
				if (value & 0x100) != 0 {
					width = uint8((value + 1) & 0xff) // new width...
					continue blockLoop                // ... next value
				}
			}

			// now expand value to signed byte
			if width < 8 {
				var shift uint8 = 8 - width
				v = int8(value << shift)
				v >>= shift
			} else {
				v = int8(value)
			}

			if err := out.WriteByte(byte(v)); err != nil {
				panic(err)
			}
			blkpos++
		}
	}
	return out.Bytes()
}

// 16-bit sample uncompressor for IT 2.14+
func uncompress16IT214(data []byte, isBigEndian bool) []byte {
	in := bytes.NewReader(data)
	out := &bytes.Buffer{}

	var (
		blklen uint16 // length of compressed data block in samples
		blkpos uint16 // position in block
		width  uint8  // actual "bit width"
		value  uint32 // value read from file to be processed
		v      int16  // sample value
		order  binary.ByteOrder

		// state for itReadbits
		bitbuf uint32
		bitnum uint32
	)

	if isBigEndian {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	// now unpack data till the dest buffer is full
	for in.Len() > 0 {
		// read a new block of compressed data and reset variables
		// block layout: word size, <size> bytes data
		bitbuf = 0
		bitnum = 0

		blklen = uint16(math.Min(0x4000, float64(in.Len())))
		blkpos = 0

		width = 17 // start with width of 17 bits

		var clen uint16
		if err := binary.Read(in, binary.LittleEndian, &clen); err != nil {
			panic(err)
		}

		// now uncompress the data block
	blockLoop:
		for blkpos < blklen {
			if width > 17 {
				// illegal width, abort
				panic(fmt.Sprintf("Illegal bit width %d for 16-bit sample\n", width))
			}
			vv, err := itReadbits(int8(width), in, &bitnum, &bitbuf)
			if err != nil {
				break blockLoop
			}
			value = vv

			if width < 7 {
				// method 1 (1-6 bits)
				// check for "100..."
				if value == 1<<(width-1) {
					// yes!
					vv, err := itReadbits(4, in, &bitnum, &bitbuf) // read new width
					if err != nil {
						break blockLoop
					}
					value = vv + 1
					if value < uint32(width) {
						width = uint8(value)
					} else {
						width = uint8(value + 1)
					}
					continue blockLoop // ... next value
				}
			} else if width < 17 {
				// method 2 (7-16 bits)
				var border uint16 = (0xFFFF >> (17 - width)) - 8 // lower border for width chg
				if value > uint32(border) && value <= uint32(border+16) {
					value -= uint32(border) // convert width to 1-16
					if value < uint32(width) {
						width = uint8(value)
					} else {
						width = uint8(value + 1)
					}
					continue blockLoop // ... next value
				}
			} else {
				// method 3 (9 bits)
				// bit 8 set?
				if (value & 0x10000) != 0 {
					width = uint8((value + 1) & 0xff) // new width...
					continue blockLoop                // ... next value
				}
			}

			// now expand value to signed byte
			if width < 8 {
				var shift uint8 = 16 - width
				v = int16(value << shift)
				v >>= shift
			} else {
				v = int16(value)
			}

			if err := binary.Write(out, order, v); err != nil {
				panic(err)
			}
			blkpos++
		}
	}
	return out.Bytes()
}

func deltaDecode(data []byte, format pcm.SampleDataFormat) {
	switch format {
	case pcm.SampleDataFormat8BitSigned, pcm.SampleDataFormat8BitUnsigned:
		deltaDecode8(data)
	case pcm.SampleDataFormat16BitLESigned, pcm.SampleDataFormat16BitLEUnsigned:
		deltaDecode16(data, binary.LittleEndian)
	case pcm.SampleDataFormat16BitBESigned, pcm.SampleDataFormat16BitBEUnsigned:
		deltaDecode16(data, binary.BigEndian)
	}
}

func deltaDecode8(data []byte) {
	old := int8(0)
	for i, s := range data {
		new := int8(s) + old
		data[i] = uint8(new)
		old = new
	}
}

func deltaDecode16(data []byte, order binary.ByteOrder) {
	old := int16(0)
	for i := 0; i < len(data); i += 2 {
		s := order.Uint16(data[i:])
		new := int16(s) + old
		order.PutUint16(data[i:], uint16(new))
		old = new
	}
}
