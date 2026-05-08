// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"

	internal_audio "github.com/rapidaai/api/assistant-api/internal/audio"
	internal_denoiser "github.com/rapidaai/api/assistant-api/internal/denoiser"
	internal_end_of_speech "github.com/rapidaai/api/assistant-api/internal/end_of_speech"
	internal_transformer "github.com/rapidaai/api/assistant-api/internal/transformer"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	internal_vad "github.com/rapidaai/api/assistant-api/internal/vad"
	"github.com/rapidaai/pkg/utils"
	"golang.org/x/sync/errgroup"
)

// Init initializes the audio talking system for a given assistant persona.
// initializeSpeechToText initializes the STT transformer only.
func (listening *genericRequestor) initializeSpeechToText(ctx context.Context) error {
	transformerConfig, _ := listening.GetSpeechToTextTransformer()
	if transformerConfig == nil {
		return nil
	}
	options := utils.MergeMaps(utils.Option{"microphone.eos.timeout": 500}, transformerConfig.GetOptions())
	credentialId, err := options.GetUint64("rapida.credential_id")
	if err != nil {
		listening.logger.Errorf("unable to find credential from options %+v", err)
		return err
	}
	credential, err := listening.VaultCaller().GetCredential(ctx, listening.Auth(), credentialId)
	if err != nil {
		listening.logger.Errorf("Api call to find credential failed %+v", err)
		return err
	}
	atransformer, err := internal_transformer.GetSpeechToTextTransformer(
		ctx,
		listening.logger,
		transformerConfig.AudioProvider,
		credential,
		func(pkt ...internal_type.Packet) error { return listening.OnPacket(ctx, pkt...) },
		options)
	if err != nil {
		listening.logger.Errorf("unable to create input audio transformer with error %v", err)
		return err
	}
	if err := atransformer.Initialize(); err != nil {
		listening.logger.Errorf("unable to initialize transformer %v", err)
		return err
	}
	listening.speechToTextTransformer = atransformer
	return nil
}

// disconnectSpeechToText closes the STT transformer only.
func (listening *genericRequestor) disconnectSpeechToText(ctx context.Context) error {
	if listening.speechToTextTransformer != nil {
		if err := listening.speechToTextTransformer.Close(ctx); err != nil {
			listening.logger.Warnf("cancel speech-to-text transformer with error %v", err)
		}
		listening.speechToTextTransformer = nil
	}
	return nil
}

// disconnectVAD closes the VAD only.
func (listening *genericRequestor) disconnectVAD(_ context.Context) error {
	if listening.vad != nil {
		if err := listening.vad.Close(); err != nil {
			listening.logger.Warnf("cancel vad with error %v", err)
		}
		listening.vad = nil
	}
	return nil
}

// disconnectDenoiser closes the denoiser only.
func (listening *genericRequestor) disconnectDenoiser(_ context.Context) error {
	if listening.denoiser != nil {
		if err := listening.denoiser.Close(); err != nil {
			listening.logger.Warnf("cancel denoiser with error %v", err)
		}
		listening.denoiser = nil
	}
	return nil
}

func (listening *genericRequestor) initializeEndOfSpeech(ctx context.Context) error {
	options := utils.Option{"microphone.eos.timeout": 500}
	transformerConfig, _ := listening.GetSpeechToTextTransformer()
	if transformerConfig != nil {
		options = utils.MergeMaps(options, transformerConfig.GetOptions())
	}
	endOfSpeech, err := internal_end_of_speech.GetEndOfSpeech(ctx,
		listening.logger,
		listening.OnPacket,
		options)
	if err != nil {
		listening.logger.Warnf("unable to initialize text analyzer %+v", err)
		return err
	}
	listening.endOfSpeech = endOfSpeech
	return nil
}

func (listening *genericRequestor) disconnectEndOfSpeech(ctx context.Context) error {
	if listening.endOfSpeech != nil {
		if err := listening.endOfSpeech.Close(); err != nil {
			listening.logger.Warnf("cancel end of speech with error %v", err)
		}
	}
	return nil
}

func (listening *genericRequestor) initializeDenoiser(ctx context.Context) error {
	options := utils.Option{"microphone.eos.timeout": 500}
	if cfg, _ := listening.GetSpeechToTextTransformer(); cfg != nil {
		options = utils.MergeMaps(options, cfg.GetOptions())
	}
	denoise, err := internal_denoiser.GetDenoiser(ctx, listening.logger, internal_audio.RAPIDA_INTERNAL_AUDIO_CONFIG,
		func(pctx context.Context, pkt ...internal_type.Packet) error { return listening.OnPacket(pctx, pkt...) },
		options)
	if err != nil {
		listening.logger.Errorf("error while initializing denoiser %+v", err)
		return err
	}
	listening.denoiser = denoise
	return nil
}

func (listening *genericRequestor) initializeVAD(ctx context.Context) error {
	options := utils.Option{"microphone.eos.timeout": 500}
	if cfg, _ := listening.GetSpeechToTextTransformer(); cfg != nil {
		options = utils.MergeMaps(options, cfg.GetOptions())
	}
	vad, err := internal_vad.GetVAD(ctx, listening.logger, listening.OnPacket, options)
	if err != nil {
		listening.logger.Errorf("error while initializing vad %+v", err)
		return err
	}
	listening.vad = vad
	return nil
}

func (spk *genericRequestor) initializeTextToSpeech(ctx context.Context) error {
	outputTransformer, _ := spk.GetTextToSpeechTransformer()
	if outputTransformer == nil {
		return nil
	}
	speakerOpts := utils.MergeMaps(outputTransformer.GetOptions())
	eGroup, ectx := errgroup.WithContext(ctx)
	eGroup.Go(func() error {
		credentialId, err := speakerOpts.GetUint64("rapida.credential_id")
		if err != nil {
			spk.logger.Errorf("tts: unable to find credential from options %+v", err)
			return err
		}
		credential, err := spk.VaultCaller().GetCredential(ectx, spk.Auth(), credentialId)
		if err != nil {
			spk.logger.Errorf("tts: api call to find credential failed %+v", err)
			return err
		}
		// Use the session ctx (not errgroup's ectx) so the transformer's stream
		// lifecycle is tied to the session, not the short-lived errgroup.
		atransformer, err := internal_transformer.GetTextToSpeechTransformer(
			ctx, spk.logger,
			outputTransformer.GetName(),
			credential,
			func(pkt ...internal_type.Packet) error { return spk.OnPacket(ctx, pkt...) },
			speakerOpts)
		if err != nil {
			spk.logger.Errorf("tts: unable to create transformer %v", err)
			return err
		}
		if err := atransformer.Initialize(); err != nil {
			spk.logger.Errorf("tts: unable to initialize transformer %v", err)
			return err
		}
		spk.textToSpeechTransformer = atransformer
		return nil
	})
	return eGroup.Wait()
}

func (spk *genericRequestor) disconnectInputNormalizer(ctx context.Context) {
	if spk.inputNormalizer != nil {
		spk.inputNormalizer.Close(ctx)
		spk.inputNormalizer = nil
	}
}

func (spk *genericRequestor) disconnectOutputNormalizer(ctx context.Context) {
	if spk.outputNormalizer != nil {
		spk.outputNormalizer.Close(ctx)
		spk.outputNormalizer = nil
	}
}

func (spk *genericRequestor) disconnectTextToSpeech(ctx context.Context) error {
	if spk.textToSpeechTransformer != nil {
		if err := spk.textToSpeechTransformer.Close(ctx); err != nil {
			spk.logger.Errorf("cancel all output transformer with error %v", err)
		}
		spk.textToSpeechTransformer = nil
	}
	return nil
}
