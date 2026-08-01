package stego

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	conversationstenography "conversationstenography"
)

const algorithmName = "conversationstenography/conversation-chain"

var ErrNotConfigured = errors.New("conversation steganography engine is not configured")

type Record struct {
	Index          uint64 `json:"index"`
	From           string `json:"from"`
	SenderSequence uint64 `json:"senderSequence"`
	CarrierText    string `json:"carrierText"`
	BroadcastType  string `json:"broadcastType,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

type EncodeRequest struct {
	ConversationID string   `json:"conversationId"`
	Sender         string   `json:"sender"`
	SecretPhrase   string   `json:"secretPhrase"`
	Plaintext      string   `json:"plaintext"`
	Records        []Record `json:"records"`
}

type EncodeResponse struct {
	ConversationID string   `json:"conversationId"`
	Sender         string   `json:"sender"`
	Plaintext      string   `json:"plaintext"`
	CarrierText    string   `json:"carrierText"`
	Record         Record   `json:"record"`
	Records        []Record `json:"records"`
	SyncCode       string   `json:"syncCode"`
	Algorithm      string   `json:"algorithm"`
}

type DecodeRequest struct {
	ConversationID string   `json:"conversationId"`
	Sender         string   `json:"sender"`
	SecretPhrase   string   `json:"secretPhrase"`
	CarrierText    string   `json:"carrierText"`
	Records        []Record `json:"records"`
}

type DecodeResponse struct {
	ConversationID string   `json:"conversationId"`
	Sender         string   `json:"sender"`
	CarrierText    string   `json:"carrierText"`
	Plaintext      string   `json:"plaintext"`
	Record         Record   `json:"record"`
	Records        []Record `json:"records"`
	SyncCode       string   `json:"syncCode"`
	Algorithm      string   `json:"algorithm"`
}

type Status struct {
	Configured       bool   `json:"configured"`
	Algorithm        string `json:"algorithm"`
	ModelFingerprint string `json:"modelFingerprint,omitempty"`
}

type Service interface {
	Encode(context.Context, EncodeRequest) (EncodeResponse, error)
	Decode(context.Context, DecodeRequest) (DecodeResponse, error)
	Status() Status
	Close() error
}

type DisabledService struct{}

func (DisabledService) Encode(context.Context, EncodeRequest) (EncodeResponse, error) {
	return EncodeResponse{}, ErrNotConfigured
}

func (DisabledService) Decode(context.Context, DecodeRequest) (DecodeResponse, error) {
	return DecodeResponse{}, ErrNotConfigured
}

func (DisabledService) Status() Status {
	return Status{Configured: false, Algorithm: algorithmName}
}

func (DisabledService) Close() error { return nil }

type ConversationService struct {
	model  conversationstenography.LanguageModel
	closer io.Closer
	config conversationstenography.GenerativeConfig
}

func NewFromEnv(ctx context.Context) (Service, error) {
	enabled, err := envBool("STEGANOGRAPHY_ENABLED", false)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return DisabledService{}, nil
	}
	command := strings.TrimSpace(os.Getenv("STEGANOGRAPHY_MODEL_COMMAND"))
	if command == "" {
		return nil, errors.New("STEGANOGRAPHY_MODEL_COMMAND is required when STEGANOGRAPHY_ENABLED=true")
	}

	args, err := parseLaunchArgs(os.Getenv("STEGANOGRAPHY_MODEL_ARGS"))
	if err != nil {
		return nil, fmt.Errorf("parse STEGANOGRAPHY_MODEL_ARGS: %w", err)
	}
	config, err := generativeConfigFromEnv()
	if err != nil {
		return nil, err
	}
	model, err := conversationstenography.NewProcessModel(ctx, command, args...)
	if err != nil {
		return nil, fmt.Errorf("launch conversation steganography model: %w", err)
	}
	service, err := NewConversationService(model, model, config)
	if err != nil {
		_ = model.Close()
		return nil, err
	}
	return service, nil
}

func NewConversationService(model conversationstenography.LanguageModel, closer io.Closer, config conversationstenography.GenerativeConfig) (*ConversationService, error) {
	if _, err := conversationstenography.NewGenerativeCodec(model, config); err != nil {
		return nil, fmt.Errorf("validate conversation steganography config: %w", err)
	}
	return &ConversationService{model: model, closer: closer, config: config}, nil
}

func (s *ConversationService) Encode(ctx context.Context, request EncodeRequest) (EncodeResponse, error) {
	conversationID, sender, err := validateCommon(request.ConversationID, request.Sender)
	if err != nil {
		return EncodeResponse{}, err
	}
	if request.Plaintext == "" {
		return EncodeResponse{}, errors.New("plaintext is required")
	}

	chain, err := s.buildChain(request.SecretPhrase, conversationID, request.Records)
	if err != nil {
		return EncodeResponse{}, err
	}
	record, err := chain.Send(ctx, sender, []byte(request.Plaintext))
	if err != nil {
		return EncodeResponse{}, fmt.Errorf("encode carrier text: %w", err)
	}
	publicRecord := fromUpstreamRecord(record)
	return EncodeResponse{
		ConversationID: conversationID,
		Sender:         sender,
		Plaintext:      request.Plaintext,
		CarrierText:    publicRecord.CarrierText,
		Record:         publicRecord,
		Records:        fromUpstreamRecords(chain.Records()),
		SyncCode:       chain.SyncCode(),
		Algorithm:      algorithmName,
	}, nil
}

func (s *ConversationService) Decode(ctx context.Context, request DecodeRequest) (DecodeResponse, error) {
	conversationID, sender, err := validateCommon(request.ConversationID, request.Sender)
	if err != nil {
		return DecodeResponse{}, err
	}
	carrierText := request.CarrierText
	if strings.TrimSpace(carrierText) == "" {
		return DecodeResponse{}, errors.New("carrierText is required")
	}

	chain, err := s.buildChain(request.SecretPhrase, conversationID, request.Records)
	if err != nil {
		return DecodeResponse{}, err
	}
	plaintext, record, err := chain.Receive(ctx, sender, carrierText)
	if err != nil {
		return DecodeResponse{}, fmt.Errorf("decode carrier text: %w", err)
	}
	publicRecord := fromUpstreamRecord(record)
	return DecodeResponse{
		ConversationID: conversationID,
		Sender:         sender,
		CarrierText:    publicRecord.CarrierText,
		Plaintext:      string(plaintext),
		Record:         publicRecord,
		Records:        fromUpstreamRecords(chain.Records()),
		SyncCode:       chain.SyncCode(),
		Algorithm:      algorithmName,
	}, nil
}

func (s *ConversationService) Status() Status {
	return Status{Configured: true, Algorithm: algorithmName, ModelFingerprint: s.model.Fingerprint()}
}

func (s *ConversationService) Close() error {
	if s.closer == nil {
		return nil
	}
	return s.closer.Close()
}

func (s *ConversationService) buildChain(secretPhrase, conversationID string, records []Record) (*conversationstenography.ConversationChain, error) {
	key, err := conversationstenography.DeriveKeyFromPhrase(secretPhrase, conversationID)
	if err != nil {
		return nil, err
	}
	chain, err := conversationstenography.NewConversationChain(s.model, key, conversationID, s.config)
	if err != nil {
		return nil, err
	}
	if err := chain.RestorePublic(toUpstreamRecords(records)); err != nil {
		return nil, fmt.Errorf("restore conversation transcript: %w", err)
	}
	return chain, nil
}

func validateCommon(conversationID, sender string) (string, string, error) {
	conversationID = strings.TrimSpace(conversationID)
	sender = strings.TrimSpace(sender)
	if conversationID == "" || len(conversationID) > 200 {
		return "", "", errors.New("a valid conversationId is required")
	}
	if sender == "" || len(sender) > 100 {
		return "", "", errors.New("a valid sender is required")
	}
	return conversationID, sender, nil
}

func toUpstreamRecords(records []Record) []conversationstenography.ChainRecord {
	converted := make([]conversationstenography.ChainRecord, len(records))
	for index, record := range records {
		converted[index] = conversationstenography.ChainRecord{
			Index: record.Index, From: record.From, SenderSequence: record.SenderSequence, Encrypted: record.CarrierText,
		}
	}
	return converted
}

func fromUpstreamRecord(record conversationstenography.ChainRecord) Record {
	return Record{Index: record.Index, From: record.From, SenderSequence: record.SenderSequence, CarrierText: record.Encrypted}
}

func fromUpstreamRecords(records []conversationstenography.ChainRecord) []Record {
	converted := make([]Record, len(records))
	for index, record := range records {
		converted[index] = fromUpstreamRecord(record)
	}
	return converted
}

func generativeConfigFromEnv() (conversationstenography.GenerativeConfig, error) {
	config := conversationstenography.GenerativeConfig{
		Prompt:            envString("STEGANOGRAPHY_PROMPT", "The weather today is"),
		ChainSystem:       envString("STEGANOGRAPHY_CHAIN_SYSTEM", ""),
		Coding:            envString("STEGANOGRAPHY_CODING", "arithmetic"),
		TopN:              8,
		Temperature:       1,
		FinishTokens:      32,
		StrictStyle:       true,
		CandidatePool:     8,
		CarrierTrials:     2,
		NaturalnessSlack:  0.35,
		SemanticThreshold: -6,
		LengthBias:        0.1,
	}
	var err error
	if config.TopN, err = envInt("STEGANOGRAPHY_TOP_N", config.TopN); err != nil {
		return config, err
	}
	if config.Temperature, err = envFloat("STEGANOGRAPHY_TEMPERATURE", config.Temperature); err != nil {
		return config, err
	}
	if config.FinishTokens, err = envInt("STEGANOGRAPHY_FINISH_TOKENS", config.FinishTokens); err != nil {
		return config, err
	}
	if config.StrictStyle, err = envBool("STEGANOGRAPHY_STRICT_STYLE", config.StrictStyle); err != nil {
		return config, err
	}
	if config.CandidatePool, err = envInt("STEGANOGRAPHY_CANDIDATE_POOL", config.CandidatePool); err != nil {
		return config, err
	}
	if config.RefreshSentences, err = envBool("STEGANOGRAPHY_REFRESH_SENTENCES", false); err != nil {
		return config, err
	}
	if config.CarrierTrials, err = envInt("STEGANOGRAPHY_CARRIER_TRIALS", config.CarrierTrials); err != nil {
		return config, err
	}
	if config.NaturalnessSlack, err = envFloat("STEGANOGRAPHY_NATURALNESS_SLACK", config.NaturalnessSlack); err != nil {
		return config, err
	}
	if config.SemanticJudge, err = envBool("STEGANOGRAPHY_SEMANTIC_JUDGE", false); err != nil {
		return config, err
	}
	if config.SemanticThreshold, err = envFloat("STEGANOGRAPHY_SEMANTIC_THRESHOLD", config.SemanticThreshold); err != nil {
		return config, err
	}
	if config.LengthBias, err = envFloat("STEGANOGRAPHY_LENGTH_BIAS", config.LengthBias); err != nil {
		return config, err
	}
	return config, nil
}

func envString(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return value, nil
}

func envFloat(name string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	return value, nil
}

func envBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}

func parseLaunchArgs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var args []string
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			return nil, errors.New("JSON argument array is invalid")
		}
		return args, nil
	}

	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	started := false
	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}
	for _, character := range raw {
		if escaped {
			current.WriteRune(character)
			escaped = false
			started = true
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				current.WriteRune(character)
			}
			continue
		}
		switch {
		case character == '\'' || character == '"':
			quote = character
			started = true
		case unicode.IsSpace(character):
			flush()
		default:
			current.WriteRune(character)
			started = true
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New("argument string contains an unfinished quote or escape")
	}
	flush()
	return args, nil
}
