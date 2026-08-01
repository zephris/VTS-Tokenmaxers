package messaging

import (
	"context"
	"errors"
	"strings"

	"vts-tokenmaxers/apps/server/internal/stego"
	"vts-tokenmaxers/apps/server/internal/transcript"
)

var ErrInvalidStation = errors.New("a valid stationId is required")

type EncodeRequest struct {
	ConversationID string `json:"conversationId"`
	StationID      string `json:"stationId"`
	Sender         string `json:"sender"`
	SecretPhrase   string `json:"secretPhrase"`
	Plaintext      string `json:"plaintext"`
}

type EncodeResponse struct {
	ConversationID   string         `json:"conversationId"`
	StationID        string         `json:"stationId"`
	Sender           string         `json:"sender"`
	Plaintext        string         `json:"plaintext"`
	CarrierText      string         `json:"carrierText"`
	Record           stego.Record   `json:"record"`
	Records          []stego.Record `json:"records"`
	SyncCode         string         `json:"syncCode"`
	Algorithm        string         `json:"algorithm"`
	ModelFingerprint string         `json:"modelFingerprint,omitempty"`
}

type DecodeRequest struct {
	ConversationID string `json:"conversationId"`
	StationID      string `json:"stationId"`
	Sender         string `json:"sender"`
	SecretPhrase   string `json:"secretPhrase"`
	CarrierText    string `json:"carrierText"`
}

type DecodeResponse struct {
	ConversationID   string         `json:"conversationId"`
	StationID        string         `json:"stationId"`
	Sender           string         `json:"sender"`
	CarrierText      string         `json:"carrierText"`
	Plaintext        string         `json:"plaintext"`
	Record           stego.Record   `json:"record"`
	Records          []stego.Record `json:"records"`
	SyncCode         string         `json:"syncCode"`
	Algorithm        string         `json:"algorithm"`
	ModelFingerprint string         `json:"modelFingerprint,omitempty"`
}

type Service struct {
	codec       stego.Service
	transcripts *transcript.Store
}

func New(codec stego.Service, transcripts *transcript.Store) *Service {
	return &Service{codec: codec, transcripts: transcripts}
}

func (s *Service) Conversation(ctx context.Context, conversationID, stationID string) (transcript.Snapshot, error) {
	if err := validateStation(stationID); err != nil {
		return transcript.Snapshot{}, err
	}
	if strings.TrimSpace(conversationID) == "" || len(strings.TrimSpace(conversationID)) > 200 {
		return transcript.Snapshot{}, errors.New("a valid conversationId is required")
	}
	snapshot, err := s.transcripts.Snapshot(ctx, strings.TrimSpace(conversationID), strings.TrimSpace(stationID))
	if err != nil {
		return transcript.Snapshot{}, err
	}
	status := s.codec.Status()
	snapshot.Algorithm = status.Algorithm
	if snapshot.ModelFingerprint == "" {
		snapshot.ModelFingerprint = status.ModelFingerprint
	}
	return snapshot, nil
}

func (s *Service) Encode(ctx context.Context, request EncodeRequest) (EncodeResponse, error) {
	if err := validateStation(request.StationID); err != nil {
		return EncodeResponse{}, err
	}
	if err := validateMessageInput(request.ConversationID, request.Sender, request.SecretPhrase); err != nil {
		return EncodeResponse{}, err
	}
	if request.Plaintext == "" || len(request.Plaintext) > 4000 {
		return EncodeResponse{}, errors.New("plaintext must contain between 1 and 4000 characters")
	}
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.StationID = strings.TrimSpace(request.StationID)
	snapshot, err := s.transcripts.Snapshot(ctx, request.ConversationID, request.StationID)
	if err != nil {
		return EncodeResponse{}, err
	}

	encoded, err := s.codec.Encode(ctx, stego.EncodeRequest{
		ConversationID: request.ConversationID,
		Sender:         request.Sender,
		SecretPhrase:   request.SecretPhrase,
		Plaintext:      request.Plaintext,
		Records:        snapshot.Records,
	})
	if err != nil {
		return EncodeResponse{}, err
	}
	status := s.codec.Status()
	if err := s.transcripts.Append(ctx, request.ConversationID, request.StationID, encoded.Record, encoded.SyncCode, status.ModelFingerprint); err != nil {
		return EncodeResponse{}, err
	}
	return EncodeResponse{
		ConversationID: encoded.ConversationID, StationID: request.StationID, Sender: encoded.Sender,
		Plaintext: encoded.Plaintext, CarrierText: encoded.CarrierText, Record: encoded.Record,
		Records: encoded.Records, SyncCode: encoded.SyncCode, Algorithm: encoded.Algorithm,
		ModelFingerprint: status.ModelFingerprint,
	}, nil
}

func (s *Service) Decode(ctx context.Context, request DecodeRequest) (DecodeResponse, error) {
	if err := validateStation(request.StationID); err != nil {
		return DecodeResponse{}, err
	}
	if err := validateMessageInput(request.ConversationID, request.Sender, request.SecretPhrase); err != nil {
		return DecodeResponse{}, err
	}
	if strings.TrimSpace(request.CarrierText) == "" || len(request.CarrierText) > 32_000 {
		return DecodeResponse{}, errors.New("carrierText must contain between 1 and 32000 characters")
	}
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.StationID = strings.TrimSpace(request.StationID)
	snapshot, err := s.transcripts.Snapshot(ctx, request.ConversationID, request.StationID)
	if err != nil {
		return DecodeResponse{}, err
	}

	decoded, err := s.codec.Decode(ctx, stego.DecodeRequest{
		ConversationID: request.ConversationID,
		Sender:         request.Sender,
		SecretPhrase:   request.SecretPhrase,
		CarrierText:    request.CarrierText,
		Records:        snapshot.Records,
	})
	if err != nil {
		return DecodeResponse{}, err
	}
	status := s.codec.Status()
	if err := s.transcripts.Append(ctx, request.ConversationID, request.StationID, decoded.Record, decoded.SyncCode, status.ModelFingerprint); err != nil {
		return DecodeResponse{}, err
	}
	return DecodeResponse{
		ConversationID: decoded.ConversationID, StationID: request.StationID, Sender: decoded.Sender,
		CarrierText: decoded.CarrierText, Plaintext: decoded.Plaintext, Record: decoded.Record,
		Records: decoded.Records, SyncCode: decoded.SyncCode, Algorithm: decoded.Algorithm,
		ModelFingerprint: status.ModelFingerprint,
	}, nil
}

func validateStation(stationID string) error {
	stationID = strings.TrimSpace(stationID)
	if stationID == "" || len(stationID) > 100 {
		return ErrInvalidStation
	}
	return nil
}

func validateMessageInput(conversationID, sender, secretPhrase string) error {
	conversationID = strings.TrimSpace(conversationID)
	sender = strings.TrimSpace(sender)
	if conversationID == "" || len(conversationID) > 200 {
		return errors.New("a valid conversationId is required")
	}
	if sender == "" || len(sender) > 100 {
		return errors.New("a valid sender is required")
	}
	if len(secretPhrase) < 16 || len(secretPhrase) > 160 {
		return errors.New("secretPhrase must contain between 16 and 160 characters")
	}
	return nil
}
