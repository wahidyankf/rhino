package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// errInjected is the failure every injected fault returns.
var errInjected = errors.New("injected failure")

// memSource is the binary the archive reads: its bytes, its declared size, and
// the faults its operations raise.
type memSource struct {
	data     []byte
	size     int64
	readErr  error
	statErr  error
	closeErr error
	closed   bool
}

func (m *memSource) Read(p []byte) (int, error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if len(m.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, m.data)
	m.data = m.data[n:]
	return n, nil
}

func (m *memSource) Stat() (fs.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	return memInfo{size: m.size}, nil
}

func (m *memSource) Close() error {
	m.closed = true
	return m.closeErr
}

// memInfo reports only a size.
type memInfo struct{ size int64 }

func (m memInfo) Name() string       { return "binary" }
func (m memInfo) Size() int64        { return m.size }
func (m memInfo) Mode() fs.FileMode  { return 0o755 }
func (m memInfo) ModTime() time.Time { return time.Time{} }
func (m memInfo) IsDir() bool        { return false }
func (m memInfo) Sys() any           { return nil }

// memDestination collects the archive bytes and reports its faults.
type memDestination struct {
	bytes.Buffer
	writeErr error
	closeErr error
	closed   bool
}

func (m *memDestination) Write(p []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.Buffer.Write(p)
}

func (m *memDestination) Close() error {
	m.closed = true
	return m.closeErr
}

// memFiles is the file system the archive writes through.
type memFiles struct {
	source      *memSource
	destination *memDestination
	openErr     error
	createErr   error
}

func (m *memFiles) Open(string) (sourceFile, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	return m.source, nil
}

func (m *memFiles) Create(string) (io.WriteCloser, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.destination, nil
}

// withFiles swaps the archive's file system for the test's double.
func withFiles(t *testing.T, files fileSystem) {
	t.Helper()
	saved := hostFiles
	hostFiles = files
	t.Cleanup(func() { hostFiles = saved })
}

// newArchiveFiles is a healthy binary of 4 bytes and an empty destination.
func newArchiveFiles() *memFiles {
	return &memFiles{
		source:      &memSource{data: []byte("ELF!"), size: 4},
		destination: &memDestination{},
	}
}

func TestWriteArchivesTheBinaryAsOneDeterministicMember(t *testing.T) {
	files := newArchiveFiles()
	withFiles(t, files)
	modified := time.Unix(1700000000, 0)
	if err := write("binary", modified, "out.tar.gz"); err != nil {
		t.Fatalf("write = %v", err)
	}
	if !files.source.closed || !files.destination.closed {
		t.Fatal("write left a file open")
	}
	gz, err := gzip.NewReader(&files.destination.Buffer)
	if err != nil {
		t.Fatalf("archive is not gzip: %v", err)
	}
	if gz.OS != unixOS {
		t.Fatalf("gzip OS = %d, want %d", gz.OS, unixOS)
	}
	reader := tar.NewReader(gz)
	header, err := reader.Next()
	if err != nil {
		t.Fatalf("archive has no member: %v", err)
	}
	if header.Name != "rhino" || header.Mode != 0o755 || header.Uname != "root" || header.Gname != "root" ||
		header.Size != 4 || !header.ModTime.Equal(modified) || header.Typeflag != tar.TypeReg {
		t.Fatalf("member header = %+v", header)
	}
	body, err := io.ReadAll(reader)
	if err != nil || string(body) != "ELF!" {
		t.Fatalf("member body = %q, %v", body, err)
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("archive has more than one member: %v", err)
	}
}

func TestWriteRefusesEachFailedStep(t *testing.T) {
	tests := []struct {
		name  string
		files func() *memFiles
	}{
		{name: "source cannot be opened", files: func() *memFiles {
			f := newArchiveFiles()
			f.openErr = errInjected
			return f
		}},
		{name: "source cannot be statted", files: func() *memFiles {
			f := newArchiveFiles()
			f.source.statErr = errInjected
			return f
		}},
		{name: "destination cannot be created", files: func() *memFiles {
			f := newArchiveFiles()
			f.createErr = errInjected
			return f
		}},
		{name: "source read fails", files: func() *memFiles {
			f := newArchiveFiles()
			f.source.readErr = errInjected
			return f
		}},
		{name: "source is shorter than its size", files: func() *memFiles {
			f := newArchiveFiles()
			f.source.size = 100
			return f
		}},
		{name: "destination write fails", files: func() *memFiles {
			f := newArchiveFiles()
			f.destination.writeErr = errInjected
			return f
		}},
		{name: "destination close fails", files: func() *memFiles {
			f := newArchiveFiles()
			f.destination.closeErr = errInjected
			return f
		}},
		{name: "source close fails", files: func() *memFiles {
			f := newArchiveFiles()
			f.source.closeErr = errInjected
			return f
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFiles(t, test.files())
			if err := write("binary", time.Unix(0, 0), "out.tar.gz"); err == nil {
				t.Fatal("write succeeded, want a failure")
			}
		})
	}
}

func TestExecuteReportsUsageAndFailuresOnStderr(t *testing.T) {
	var stderr strings.Builder
	if code := execute([]string{"-binary", "only"}, &stderr); code != 1 {
		t.Fatalf("usage exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "release-archive:") {
		t.Fatalf("usage stderr = %q", stderr.String())
	}

	withFiles(t, &memFiles{openErr: errInjected})
	stderr.Reset()
	args := []string{"-binary", "binary", "-mtime", "0", "-output", "out.tar.gz"}
	if code := execute(args, &stderr); code != 1 || !strings.Contains(stderr.String(), "injected failure") {
		t.Fatalf("failed write exit = %d, stderr %q", code, stderr.String())
	}
}

func TestExecuteSucceedsForAValidInvocation(t *testing.T) {
	withFiles(t, newArchiveFiles())
	var stderr strings.Builder
	args := []string{"-binary", "binary", "-mtime", "1700000000", "-output", "out.tar.gz"}
	if code := execute(args, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stderr %q; want 0 and silence", code, stderr.String())
	}
}

func TestRunRefusesEachMalformedInvocation(t *testing.T) {
	withFiles(t, newArchiveFiles())
	refused := map[string][]string{
		"no flags":       {},
		"no binary":      {"-mtime", "0", "-output", "x"},
		"no output":      {"-binary", "b", "-mtime", "0"},
		"negative mtime": {"-binary", "b", "-mtime", "-1", "-output", "x"},
		"positional":     {"-binary", "b", "-mtime", "0", "-output", "x", "extra"},
		"unknown flag":   {"-nope"},
	}
	for name, args := range refused {
		t.Run(name, func(t *testing.T) {
			if err := run(args, io.Discard); err == nil {
				t.Fatalf("run(%v) succeeded, want a refusal", args)
			}
		})
	}
}
