package stego

import (
	"context"
	"reflect"
	"strings"
	"testing"

	conversationstenography "conversationstenography"
)

type deterministicModel struct{}

func (deterministicModel) Fingerprint() string { return "server-test-v1" }

func (deterministicModel) Tokenize(_ context.Context, text string) ([]int, error) {
	ids := make([]int, 0, len(text))
	for _, character := range text {
		if character >= 'a' && character <= 'h' {
			ids = append(ids, int(character-'a'))
		} else {
			ids = append(ids, 1000+int(character))
		}
	}
	return ids, nil
}

func (deterministicModel) Detokenize(_ context.Context, ids []int) (string, error) {
	characters := make([]rune, len(ids))
	for index, id := range ids {
		characters[index] = rune('a' + id)
	}
	return string(characters), nil
}

func (deterministicModel) Next(_ context.Context, tokens []int, topN int) ([]conversationstenography.TokenCandidate, error) {
	candidates := make([]conversationstenography.TokenCandidate, topN)
	for index := range candidates {
		candidates[index] = conversationstenography.TokenCandidate{
			ID:      (len(tokens) + index) % topN,
			LogProb: float64(topN - index),
			Text:    string(rune('a' + (len(tokens)+index)%topN)),
		}
	}
	return candidates, nil
}

func TestParseLaunchArgs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "plain", raw: "backend.py --model gpt2", want: []string{"backend.py", "--model", "gpt2"}},
		{name: "quoted", raw: `backend.py --model "models/tiny chat"`, want: []string{"backend.py", "--model", "models/tiny chat"}},
		{name: "json", raw: `["backend.py","--model","models/tiny chat"]`, want: []string{"backend.py", "--model", "models/tiny chat"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseLaunchArgs(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseLaunchArgs() = %#v; want %#v", got, test.want)
			}
		})
	}
}

func TestParseLaunchArgsRejectsUnfinishedQuote(t *testing.T) {
	if _, err := parseLaunchArgs(`backend.py "unfinished`); err == nil {
		t.Fatal("parseLaunchArgs() accepted an unfinished quote")
	}
}

func TestNewFromEnvDefaultsToDisabledAndIgnoresModelConfiguration(t *testing.T) {
	t.Setenv("STEGANOGRAPHY_ENABLED", "false")
	t.Setenv("STEGANOGRAPHY_MODEL_COMMAND", "/definitely/not/a/model")
	t.Setenv("STEGANOGRAPHY_MODEL_ARGS", `"unfinished`)
	service, err := NewFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if service.Status().Configured {
		t.Fatal("disabled service reported itself configured")
	}
}

func TestNewFromEnvRequiresCommandWhenEnabled(t *testing.T) {
	t.Setenv("STEGANOGRAPHY_ENABLED", "true")
	t.Setenv("STEGANOGRAPHY_MODEL_COMMAND", "")
	if _, err := NewFromEnv(context.Background()); err == nil || !strings.Contains(err.Error(), "STEGANOGRAPHY_MODEL_COMMAND") {
		t.Fatalf("error = %v; want required command error", err)
	}
}

func TestConversationServiceRoundTrip(t *testing.T) {
	service, err := NewConversationService(deterministicModel{}, nil, conversationstenography.GenerativeConfig{
		Prompt: "Test prompt", TopN: 8, Coding: "uniform", Temperature: 1, StrictStyle: false, CarrierTrials: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := service.Encode(context.Background(), EncodeRequest{
		ConversationID: "test-chat",
		Sender:         "alice",
		SecretPhrase:   "a sufficiently long shared phrase",
		Plaintext:      "meet at six",
	})
	if err != nil {
		t.Fatal(err)
	}
	if encoded.CarrierText == "" || strings.Contains(encoded.CarrierText, encoded.Plaintext) {
		t.Fatalf("unexpected carrier text %q", encoded.CarrierText)
	}
	decoded, err := service.Decode(context.Background(), DecodeRequest{
		ConversationID: "test-chat",
		Sender:         "alice",
		SecretPhrase:   "a sufficiently long shared phrase",
		CarrierText:    encoded.CarrierText,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Plaintext != encoded.Plaintext {
		t.Fatalf("decoded plaintext = %q; want %q", decoded.Plaintext, encoded.Plaintext)
	}
	if decoded.SyncCode != encoded.SyncCode {
		t.Fatalf("decode sync code = %q; want %q", decoded.SyncCode, encoded.SyncCode)
	}
}
