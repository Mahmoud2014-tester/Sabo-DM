package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

func TestJobRunUsesParallelHTTPRanges(t *testing.T) {
	data := make([]byte, 17<<20) // adaptive planner should choose three 8 MiB-target segments
	for i := range data {
		data[i] = byte((i * 31) % 251)
	}
	var rangeRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			t.Errorf("expected a Range request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var start, end int64
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= int64(len(data)) {
			t.Errorf("invalid Range header %q", rangeHeader)
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		rangeRequests.Add(1)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[start : end+1])
	}))
	defer server.Close()

	dir := t.TempDir()
	var callbackMu sync.Mutex
	var lastSegments []Segment
	job := &Job{
		URL: server.URL, Dir: dir, Name: "parallel.bin", Conns: 8,
		OnSegments: func(segments []Segment) {
			callbackMu.Lock()
			lastSegments = append([]Segment(nil), segments...)
			callbackMu.Unlock()
		},
	}
	if err := job.Run(context.Background(), nil); err != nil {
		t.Fatalf("Job.Run failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "parallel.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded file contents differ from source")
	}
	if got := rangeRequests.Load(); got < 4 { // one probe plus at least three data ranges
		t.Fatalf("expected probe plus three parallel ranges, got %d range requests", got)
	}
	callbackMu.Lock()
	defer callbackMu.Unlock()
	if len(lastSegments) != 3 {
		t.Fatalf("expected three live segments, got %d", len(lastSegments))
	}
}

func TestJobRunFallsBackWhenServerIgnoresRanges(t *testing.T) {
	data := bytes.Repeat([]byte("sabo-dm-single-stream-"), 64*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately ignore Range and return the full object as HTTP 200.
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	}))
	defer server.Close()

	dir := t.TempDir()
	var callbackMu sync.Mutex
	var lastSegments []Segment
	job := &Job{
		URL: server.URL, Dir: dir, Name: "single.bin", Conns: 32,
		OnSegments: func(segments []Segment) {
			callbackMu.Lock()
			lastSegments = append([]Segment(nil), segments...)
			callbackMu.Unlock()
		},
	}
	if err := job.Run(context.Background(), nil); err != nil {
		t.Fatalf("Job.Run single-stream fallback failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "single.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("single-stream fallback contents differ from source")
	}
	callbackMu.Lock()
	defer callbackMu.Unlock()
	if len(lastSegments) != 1 {
		t.Fatalf("expected a single segment on a server without range support, got %d", len(lastSegments))
	}
}

func TestValidResumeStateRejectsTooManySegments(t *testing.T) {
	segments := make([]Segment, 33)
	for i := range segments {
		segments[i] = Segment{Start: int64(i), End: int64(i), Done: 0}
	}
	st := &State{Size: 33, Segments: segments}
	if validResumeState(st, 33, true) {
		t.Fatal("resume state with more than 32 segments must be rejected")
	}
}
