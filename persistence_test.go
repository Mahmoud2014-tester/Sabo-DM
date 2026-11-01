package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteQueueStoreRoundTrip(t *testing.T) {
	store, err := openQueueStore(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}
	defer store.close()

	want := []savedItem{
		{URL: "https://example.test/archive.zip", Dir: `C:\Downloads`, Name: "archive.zip", Kind: kindFile, DoneBytes: 128, Total: 1024, Status: stDownloading},
		{URL: "magnet:?xt=urn:btih:0123456789abcdef", Dir: `C:\Downloads`, Kind: kindTorrent, Status: stPaused, Select: "1,2"},
	}
	if err := store.replace(want); err != nil {
		t.Fatalf("save queue snapshot: %v", err)
	}
	got, err := store.load()
	if err != nil {
		t.Fatalf("load queue snapshot: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queue round trip mismatch\nwant: %#v\n got: %#v", want, got)
	}

	// An empty snapshot must also be committed atomically and remove stale rows.
	if err := store.replace(nil); err != nil {
		t.Fatalf("clear queue snapshot: %v", err)
	}
	got, err = store.load()
	if err != nil {
		t.Fatalf("load cleared queue: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty queue after clear, got %d rows", len(got))
	}
}
