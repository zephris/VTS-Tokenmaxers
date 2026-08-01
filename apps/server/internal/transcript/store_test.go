package transcript

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"

	"vts-tokenmaxers/apps/server/internal/stego"
)

func testStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := New(db)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store, db
}

func TestAppendAndReadPublicTranscript(t *testing.T) {
	store, _ := testStore(t)
	record := stego.Record{
		Index: 0, From: "Marv", SenderSequence: 0, CarrierText: "The weather today is calm.",
		BroadcastType: "situation_report", CreatedAt: "2026-08-01T08:30:00Z",
	}
	if err := store.Append(context.Background(), "delta-channel", "sunken-command", record, "a1b2", "model-v1"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.Snapshot(context.Background(), "delta-channel", "sunken-command")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0] != record {
		t.Fatalf("snapshot records = %#v; want %#v", snapshot.Records, []stego.Record{record})
	}
	if snapshot.SyncCode != "a1b2" || snapshot.ModelFingerprint != "model-v1" {
		t.Fatalf("unexpected transcript metadata: %#v", snapshot)
	}
}

func TestAppendRejectsSequenceConflict(t *testing.T) {
	store, _ := testStore(t)
	err := store.Append(context.Background(), "delta-channel", "outpost-delta", stego.Record{Index: 1, From: "Delta", CarrierText: "carrier"}, "", "")
	if !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("Append() error = %v; want ErrSequenceConflict", err)
	}
}

func TestTranscriptSchemaDoesNotStoreSecretsOrPlaintext(t *testing.T) {
	_, db := testStore(t)
	rows, err := db.Query("PRAGMA table_info(steganography_records)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "secret_phrase" || name == "plaintext" {
			t.Fatalf("sensitive column %q must not exist", name)
		}
	}
}
