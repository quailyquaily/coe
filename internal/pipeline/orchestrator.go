package pipeline

import (
	"context"
	"fmt"
	"strings"

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
	ByteCount         int
	Transcript        string
	TranscriptWarning string
	Corrected         string
	CorrectionWarning string
	Output            output.Delivery
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
	result := Result{
		ByteCount: capture.ByteCount,
	}
	if capture.ByteCount == 0 {
		return result, nil
	}

	transcribed, err := o.ASR.Transcribe(ctx, capture)
	if err != nil {
		return Result{}, err
	}
	result.Transcript = transcribed.Text
	if strings.TrimSpace(transcribed.Warning) != "" {
		result.TranscriptWarning = transcribed.Warning
	}
	if strings.TrimSpace(result.Transcript) == "" {
		if result.TranscriptWarning == "" {
			result.TranscriptWarning = "ASR returned empty transcript; skipped correction and output"
		}
		return result, nil
	}

	corrected, err := o.Corrector.Correct(ctx, transcribed.Text)
	if err != nil {
		result.CorrectionWarning = err.Error()
		result.Corrected = transcribed.Text
	} else {
		result.Corrected = corrected.Text
	}
	if strings.TrimSpace(result.Corrected) == "" {
		result.Corrected = transcribed.Text
		if result.CorrectionWarning == "" {
			result.CorrectionWarning = "correction returned empty text; fell back to transcript"
		}
	}

	if o.Output != nil {
		delivery, err := o.Output.Deliver(ctx, result.Corrected)
		if err != nil {
			return Result{}, err
		}
		result.Output = delivery
	}

	return result, nil
}
