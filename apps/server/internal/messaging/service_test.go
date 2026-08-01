package messaging

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"vts-tokenmaxers/apps/server/internal/stego"
	"vts-tokenmaxers/apps/server/internal/transcript"
)

type fakeCodec struct{}

func (fakeCodec) Encode(_ context.Context, request stego.EncodeRequest) (stego.EncodeResponse, error) {
	record := stego.Record{Index: uint64(len(request.Records)), From: request.Sender, CarrierText: "carrier:" + request.Plaintext}
	records := append(append([]stego.Record{}, request.Records...), record)
	return stego.EncodeResponse{
		ConversationID: request.ConversationID, Sender: request.Sender, Plaintext: request.Plaintext,
		CarrierText: record.CarrierText, Record: record, Records: records, SyncCode: "sync", Algorithm: "fake",
	}, nil
}

func (fakeCodec) Decode(_ context.Context, request stego.DecodeRequest) (stego.DecodeResponse, error) {
	record := stego.Record{Index: uint64(len(request.Records)), From: request.Sender, CarrierText: request.CarrierText}
	records := append(append([]stego.Record{}, request.Records...), record)
	return stego.DecodeResponse{
		ConversationID: request.ConversationID, Sender: request.Sender, CarrierText: request.CarrierText,
		Plaintext: "decoded", Record: record, Records: records, SyncCode: "sync", Algorithm: "fake",
	}, nil
}

func (fakeCodec) Status() stego.Status {
	return stego.Status{Configured: true, Algorithm: "fake", ModelFingerprint: "fake-v1"}
}

func (fakeCodec) Close() error { return nil }

func TestServicePersistsOnlyPublicRecordsPerStation(t *testing.T) {
	db, err := sql.Open("sqlite", "file:messaging-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	transcripts := transcript.New(db)
	if err := transcripts.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := New(fakeCodec{}, transcripts)

	encoded, err := service.Encode(context.Background(), EncodeRequest{
		ConversationID: "delta", StationID: "command", Sender: "Marv",
		SecretPhrase: "a sufficiently long secret", Plaintext: "dispatch scout",
		BroadcastType: "situation_report",
	})
	if err != nil {
		t.Fatal(err)
	}
	if encoded.StationID != "command" || encoded.ModelFingerprint != "fake-v1" {
		t.Fatalf("unexpected response: %#v", encoded)
	}
	if encoded.Record.BroadcastType != "situation_report" || encoded.Record.CreatedAt == "" {
		t.Fatalf("missing public broadcast metadata: %#v", encoded.Record)
	}

	snapshot, err := service.Conversation(context.Background(), "delta", "command")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].CarrierText != "carrier:dispatch scout" {
		t.Fatalf("unexpected transcript: %#v", snapshot)
	}
	if snapshot.Records[0].BroadcastType != "situation_report" || snapshot.Records[0].CreatedAt == "" {
		t.Fatalf("missing persisted broadcast metadata: %#v", snapshot.Records[0])
	}
	other, err := service.Conversation(context.Background(), "delta", "outpost")
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Records) != 0 {
		t.Fatalf("station transcripts leaked: %#v", other)
	}
}
