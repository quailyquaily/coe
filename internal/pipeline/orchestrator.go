package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"coe/internal/asr"
	"coe/internal/audio"
	"coe/internal/llm"
	"coe/internal/output"
)

type Orchestrator struct {
	Recorder  audio.Recorder
	ASR       asr.Client
	Corrector llm.Corrector
	Output    *output.Coordinator
}

type Result struct {
	ByteCount          int
	AudioActivity      audio.Activity
	Transcript         string
	TranscriptWarning  string
	Corrected          string
	CorrectionWarning  string
	Output             output.Delivery
	ASRDuration        time.Duration
	CorrectionDuration time.Duration
	OutputDuration     time.Duration
	TotalDuration      time.Duration
}

func (o Orchestrator) Summary() string {
	outputSummary := "disabled"
	if o.Output != nil {
		outputSummary = o.Output.Summary()
	}
	return fmt.Sprintf(
		"recorder=%s, asr=%s, llm=%s, output={%s}",
		o.Recorder.Summary(),
		o.ASR.Name(),
		o.Corrector.Name(),
		outputSummary,
	)
}

func (o Orchestrator) ProcessCapture(ctx context.Context, capture audio.Result) (Result, error) {
	startedAt := time.Now()
	result, err := o.TranscribeCapture(ctx, capture)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(result.Transcript) == "" {
		result.TotalDuration = time.Since(startedAt)
		return result, nil
	}

	result = o.ApplyCorrection(ctx, result, o.Corrector)

	if o.Output != nil {
		result, err = o.DeliverResult(ctx, result)
		if err != nil {
			return Result{}, err
		}
	}

	result.TotalDuration = time.Since(startedAt)
	return result, nil
}

func (o Orchestrator) TranscribeCapture(ctx context.Context, capture audio.Result) (Result, error) {
	result := Result{
		ByteCount: capture.ByteCount,
	}
	if capture.ByteCount == 0 {
		return result, nil
	}

	result.AudioActivity = audio.AnalyzeActivity(capture, audio.DefaultActivityThresholds())
	if result.AudioActivity.Supported && result.AudioActivity.ApproxSilent {
		result.TranscriptWarning = "captured audio is near-silent; skipped transcription"
		return result, nil
	}
	if result.AudioActivity.Supported && result.AudioActivity.ApproxCorrupt {
		result.TranscriptWarning = "captured audio appears saturated or corrupted; skipped transcription"
		return result, nil
	}

	asrStartedAt := time.Now()
	transcribed, err := o.ASR.Transcribe(ctx, capture)
	result.ASRDuration = time.Since(asrStartedAt)
	if err != nil {
		return Result{}, err
	}
	result.Transcript = transcribed.Text
	if strings.TrimSpace(transcribed.Warning) != "" {
		result.TranscriptWarning = transcribed.Warning
	}
	if strings.TrimSpace(result.Transcript) == "" && result.TranscriptWarning == "" {
		result.TranscriptWarning = "ASR returned empty transcript; skipped correction and output"
	}

	return result, nil
}

func (o Orchestrator) ApplyCorrection(ctx context.Context, result Result, corrector llm.Corrector) Result {
	if strings.TrimSpace(result.Transcript) == "" || corrector == nil {
		return result
	}

	correctionStartedAt := time.Now()
	corrected, err := corrector.Correct(ctx, result.Transcript)
	result.CorrectionDuration = time.Since(correctionStartedAt)
	if err != nil {
		result.CorrectionWarning = err.Error()
		result.Corrected = result.Transcript
	} else {
		result.Corrected = corrected.Text
	}
	if strings.TrimSpace(result.Corrected) == "" {
		result.Corrected = result.Transcript
		if result.CorrectionWarning == "" {
			result.CorrectionWarning = "correction returned empty text; fell back to transcript"
		}
	}

	return result
}

func (o Orchestrator) DeliverResult(ctx context.Context, result Result) (Result, error) {
	if o.Output == nil || strings.TrimSpace(result.Corrected) == "" {
		return result, nil
	}

	outputStartedAt := time.Now()
	delivery, err := o.Output.Deliver(ctx, result.Corrected)
	result.OutputDuration = time.Since(outputStartedAt)
	if err != nil {
		return Result{}, err
	}
	result.Output = delivery
	return result, nil
}
