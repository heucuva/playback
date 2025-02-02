package voice

import (
	"errors"
	"fmt"

	"github.com/gotracker/gomixing/panning"
	"github.com/gotracker/gomixing/volume"
	"github.com/gotracker/playback/filter"
	itFilter "github.com/gotracker/playback/format/it/filter"
	itOscillator "github.com/gotracker/playback/format/it/oscillator"
	itPanning "github.com/gotracker/playback/format/it/panning"
	itVolume "github.com/gotracker/playback/format/it/volume"
	"github.com/gotracker/playback/frequency"
	"github.com/gotracker/playback/instrument"
	"github.com/gotracker/playback/period"
	"github.com/gotracker/playback/voice"
	"github.com/gotracker/playback/voice/autovibrato"
	"github.com/gotracker/playback/voice/component"
	"github.com/gotracker/playback/voice/fadeout"
)

type Period interface {
	period.Period
}

type itVoice[TPeriod Period] struct {
	inst       *instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]
	background bool

	pitchAndFilterEnvShared bool
	filterEnvActive         bool // if pitchAndFilterEnvShared is true, this dictates which is active initially - true=filter, false=pitch
	fadeoutMode             fadeout.Mode

	component.KeyModulator

	stopped     bool
	voicer      component.Voicer[TPeriod, itVolume.FineVolume, itVolume.Volume]
	amp         component.AmpModulator[itVolume.FineVolume, itVolume.Volume]
	fadeout     component.FadeoutModulator
	freq        component.FreqModulator[TPeriod]
	autoVibrato component.AutoVibratoModulator[TPeriod]
	pan         component.PanModulator[itPanning.Panning]
	pitchPan    component.PitchPanModulator[itPanning.Panning]
	volEnv      component.VolumeEnvelope[itVolume.Volume]
	pitchEnv    component.PitchEnvelope
	panEnv      component.PanEnvelope[itPanning.Panning]
	filterEnv   component.FilterEnvelope
	vol0Opt     component.Vol0Optimization
	voiceFilter filter.Filter

	// finals
	finalVol    volume.Volume
	finalPeriod TPeriod
	finalPan    panning.Position
}

var (
	_ voice.Sampler                                                                   = (*itVoice[period.Linear])(nil)
	_ voice.AmpModulator[itVolume.FineVolume, itVolume.FineVolume, itVolume.Volume]   = (*itVoice[period.Linear])(nil)
	_ voice.FadeoutModulator                                                          = (*itVoice[period.Linear])(nil)
	_ voice.FreqModulator[period.Linear]                                              = (*itVoice[period.Linear])(nil)
	_ voice.PanModulator[itPanning.Panning]                                           = (*itVoice[period.Linear])(nil)
	_ voice.PitchPanModulator[itPanning.Panning]                                      = (*itVoice[period.Linear])(nil)
	_ voice.VolumeEnvelope[itVolume.FineVolume, itVolume.FineVolume, itVolume.Volume] = (*itVoice[period.Linear])(nil)
	_ voice.PitchEnvelope[period.Linear]                                              = (*itVoice[period.Linear])(nil)
	_ voice.PanEnvelope[itPanning.Panning]                                            = (*itVoice[period.Linear])(nil)
	_ voice.FilterEnvelope                                                            = (*itVoice[period.Linear])(nil)
)

func New[TPeriod Period](config voice.VoiceConfig[TPeriod, itVolume.FineVolume, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]) voice.RenderVoice[TPeriod, itVolume.FineVolume, itVolume.FineVolume, itVolume.Volume, itPanning.Panning] {
	v := &itVoice[TPeriod]{
		pitchAndFilterEnvShared: true,
	}

	v.KeyModulator.Setup(component.KeyModulatorSettings{
		Attack:          v.doAttack,
		Release:         v.doRelease,
		Fadeout:         v.doFadeout,
		DeferredAttack:  v.doDeferredAttack,
		DeferredRelease: v.doDeferredRelease,
	})

	v.amp.Setup(component.AmpModulatorSettings[itVolume.FineVolume, itVolume.Volume]{
		Active:              true,
		DefaultMixingVolume: config.InitialMixing,
		DefaultVolume:       config.InitialVolume,
	})

	v.freq.Setup(component.FreqModulatorSettings[TPeriod]{
		PC: config.PC,
	})

	v.pan.Setup(component.PanModulatorSettings[itPanning.Panning]{
		Enabled:    config.PanEnabled,
		InitialPan: config.InitialPan,
	})

	v.vol0Opt.Setup(config.Vol0Optimization)

	return v
}

func (v *itVoice[TPeriod]) doAttack() {
	v.vol0Opt.Reset()
	v.autoVibrato.Reset()

	v.SetVolumeEnvelopePosition(0)
	v.SetPitchEnvelopePosition(0)
	v.SetPanEnvelopePosition(0)
	v.SetFilterEnvelopePosition(0)

	v.fadeout.Reset()
	v.volEnv.Attack()
	v.pitchEnv.Attack()
	v.panEnv.Attack()
	v.filterEnv.Attack()
	if v.voicer != nil {
		v.voicer.Attack()
	}
	v.updateFinal()
}

func (v *itVoice[TPeriod]) doRelease() {
	v.volEnv.Release()
	v.pitchEnv.Release()
	v.panEnv.Release()
	v.filterEnv.Release()
	if v.voicer != nil {
		v.voicer.Release()
	}
	if v.background && !v.volEnv.CanLoop() {
		v.KeyModulator.Fadeout() // triggers updateFinal
	} else {
		v.updateFinal()
	}
}

func (v *itVoice[TPeriod]) doFadeout() {
	if v.voicer != nil {
		v.voicer.Fadeout()
	}
	v.updateFinal()
}

func (v *itVoice[TPeriod]) doDeferredAttack() {
	if v.voicer != nil {
		v.voicer.DeferredAttack()
	}
}

func (v *itVoice[TPeriod]) doDeferredRelease() {
	if v.voicer != nil {
		v.voicer.DeferredRelease()
	}
}

func (v itVoice[TPeriod]) getFadeoutEnabled() bool {
	return v.fadeoutMode.IsFadeoutActive(v.IsKeyFadeout(), v.volEnv.IsEnabled(), v.volEnv.IsDone())
}

func (v *itVoice[TPeriod]) SetPlaybackRate(outputRate frequency.Frequency) error {
	if v.voiceFilter != nil {
		v.voiceFilter.SetPlaybackRate(outputRate)
	}
	return nil
}

func (v *itVoice[TPeriod]) Setup(inst *instrument.Instrument[TPeriod, itVolume.FineVolume, itVolume.Volume, itPanning.Panning]) error {
	v.inst = inst

	v.voicer = nil
	switch d := inst.GetData().(type) {
	case *instrument.PCM[itVolume.FineVolume, itVolume.Volume, itPanning.Panning]:
		v.filterEnvActive = d.PitchFiltMode
		v.fadeoutMode = d.FadeOut.Mode

		v.fadeout.Setup(component.FadeoutModulatorSettings{
			Enabled:   d.FadeOut.Mode != fadeout.ModeDisabled,
			GetActive: v.getFadeoutEnabled,
			Amount:    d.FadeOut.Amount,
		})

		v.pitchPan.Setup(component.PitchPanModulatorSettings[itPanning.Panning]{
			PitchPanEnable:     d.PitchPan.Enabled,
			PitchPanCenter:     d.PitchPan.Center,
			PitchPanSeparation: d.PitchPan.Separation,
		})

		volEnvSettings := component.EnvelopeSettings[itVolume.Volume, itVolume.Volume]{
			Envelope: d.VolEnv,
		}
		if d.VolEnvFinishFadesOut {
			volEnvSettings.OnFinished = func(v voice.Voice) {
				v.Fadeout()
			}
		}
		v.volEnv.Setup(volEnvSettings)

		v.pitchEnv.Setup(component.EnvelopeSettings[int8, period.Delta]{
			Envelope: d.PitchFiltEnv,
		})

		v.panEnv.Setup(component.EnvelopeSettings[itPanning.Panning, itPanning.Panning]{
			Envelope: d.PanEnv,
		})

		v.filterEnv.Setup(component.EnvelopeSettings[int8, uint8]{
			Envelope: d.PitchFiltEnv,
		})

		if err := v.amp.SetMixingVolumeOverride(d.MixingVolume); err != nil {
			return err
		}

		var s component.Sampler[TPeriod, itVolume.FineVolume, itVolume.Volume]
		s.Setup(component.SamplerSettings[TPeriod, itVolume.FineVolume, itVolume.Volume]{
			Sample:        d.Sample,
			DefaultVolume: inst.GetDefaultVolume(),
			MixVolume:     itVolume.MaxItFineVolume,
			WholeLoop:     d.Loop,
			SustainLoop:   d.SustainLoop,
		})
		v.voicer = &s

	default:
		return fmt.Errorf("unhandled instrument type: %T", d)
	}
	if inst == nil {
		return errors.New("instrument is nil")
	}

	v.autoVibrato.Setup(autovibrato.AutoVibratoSettings[TPeriod]{
		AutoVibratoConfig: inst.Static.AutoVibrato,
		Factory:           itOscillator.OscillatorFactory,
	})

	info := inst.GetVoiceFilterInfo()
	f, err := itFilter.Factory(info.Name, inst.SampleRate, info.Params)
	if err != nil {
		return fmt.Errorf("filter factory(%q) error: %w", info.Name, err)
	}
	v.voiceFilter = f

	v.Reset()
	return nil
}

func (v *itVoice[TPeriod]) Reset() error {
	v.KeyModulator.Release()
	v.stopped = false
	return errors.Join(
		v.amp.Reset(),
		v.fadeout.Reset(),
		v.freq.Reset(),
		v.autoVibrato.Reset(),
		v.pan.Reset(),
		v.pitchPan.Reset(),
		v.volEnv.Reset(),
		v.pitchEnv.Reset(),
		v.panEnv.Reset(),
		v.filterEnv.Reset(),
		v.vol0Opt.Reset(),
		v.updateFinal(),
	)
}

func (v *itVoice[TPeriod]) Stop() {
	v.stopped = true
	v.updateFinal()
}

func (v itVoice[TPeriod]) IsDone() bool {
	if v.voicer == nil || v.stopped {
		return true
	}

	if v.fadeout.IsActive() {
		return v.fadeout.GetVolume() <= 0
	}

	return v.vol0Opt.IsDone()
}

func (v *itVoice[TPeriod]) SetMuted(muted bool) error {
	return v.amp.SetMuted(muted)
}

func (v itVoice[TPeriod]) IsMuted() bool {
	return v.amp.IsMuted()
}

func (v *itVoice[TPeriod]) Tick() error {
	v.fadeout.Advance()
	v.autoVibrato.Advance()
	v.pitchPan.Advance()
	if v.IsVolumeEnvelopeEnabled() {
		if doneCB := v.volEnv.Advance(); doneCB != nil {
			doneCB(v)
		}
	}
	if v.IsPanEnvelopeEnabled() {
		if doneCB := v.panEnv.Advance(); doneCB != nil {
			doneCB(v)
		}
	}
	if v.IsPitchEnvelopeEnabled() {
		if doneCB := v.pitchEnv.Advance(); doneCB != nil {
			doneCB(v)
		}
	}
	if v.IsFilterEnvelopeEnabled() {
		if doneCB := v.filterEnv.Advance(); doneCB != nil {
			doneCB(v)
		}
	}

	if v.voiceFilter != nil && v.IsFilterEnvelopeEnabled() {
		fval := v.GetCurrentFilterEnvelope()
		v.voiceFilter.UpdateEnv(fval)
	}

	// has to be after the mod/env updates
	v.KeyModulator.DeferredUpdate()

	v.KeyModulator.Advance()

	return v.updateFinal()
}

func (v *itVoice[TPeriod]) RowEnd() error {
	v.vol0Opt.ObserveVolume(v.GetFinalVolume())
	return nil
}

func (v *itVoice[TPeriod]) Clone(background bool) voice.Voice {
	vv := itVoice[TPeriod]{
		inst:                    v.inst,
		background:              background,
		pitchAndFilterEnvShared: v.pitchAndFilterEnvShared,
		filterEnvActive:         v.filterEnvActive,
		fadeoutMode:             v.fadeoutMode,
		stopped:                 v.stopped,
		amp:                     v.amp.Clone(),
		fadeout:                 v.fadeout.Clone(),
		freq:                    v.freq.Clone(),
		autoVibrato:             v.autoVibrato.Clone(),
		pan:                     v.pan.Clone(),
		pitchPan:                v.pitchPan.Clone(),
		pitchEnv:                v.pitchEnv.Clone(nil),
		panEnv:                  v.panEnv.Clone(nil),
		filterEnv:               v.filterEnv.Clone(nil),
		vol0Opt:                 v.vol0Opt.Clone(),
	}

	vv.volEnv = v.volEnv.Clone(func(v voice.Voice) {
		vv.Fadeout()
	})

	vv.KeyModulator = v.KeyModulator.Clone(component.KeyModulatorSettings{
		Attack:          vv.doAttack,
		Release:         vv.doRelease,
		Fadeout:         vv.doFadeout,
		DeferredAttack:  vv.doDeferredAttack,
		DeferredRelease: vv.doDeferredRelease,
	})

	if v.voicer != nil {
		vv.voicer = v.voicer.Clone()
	}

	if v.voiceFilter != nil {
		vv.voiceFilter = v.voiceFilter.Clone()
	}

	return &vv
}

func (v *itVoice[TPeriod]) updateFinal() error {
	if v.IsDone() {
		v.finalVol = 0
		return nil
	}

	// volume
	vol := v.amp.GetFinalVolume()
	volEnv := volume.Volume(1)
	if v.IsVolumeEnvelopeEnabled() {
		volEnv = v.GetCurrentVolumeEnvelope().ToVolume()
	}
	fadeVol := v.fadeout.GetFinalVolume()

	v.finalVol = vol * volEnv * fadeVol

	// period
	p, err := v.freq.GetFinalPeriod()
	if err != nil {
		return err
	}
	if v.IsPitchEnvelopeEnabled() {
		delta := v.GetCurrentPitchEnvelope()
		p, err = v.inst.Static.PC.AddDelta(p, delta)
		if err != nil {
			return err
		}
	}
	v.finalPeriod = p

	// panning
	if !v.IsPanEnvelopeEnabled() {
		v.finalPan = v.pan.GetFinalPan()
	} else {
		envPan := v.panEnv.GetCurrentValue()
		v.finalPan = v.pitchPan.GetSeparatedPan(envPan).ToPosition()
	}
	return err
}
