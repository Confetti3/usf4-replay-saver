//go:build windows

package main

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// writeSlot writes a minimal replay into slot n with its checksum file.
func writeSlot(t *testing.T, dir string, n int, stamp time.Time, fill byte) {
	t.Helper()
	data := make([]byte, 0x80)
	copy(data, replayMagic)
	ft := uint64(stamp.UnixNano()/100) + 116444736000000000
	binary.LittleEndian.PutUint64(data[headerTimeOff:], ft)
	for i := 0x40; i < len(data); i++ {
		data[i] = fill
	}
	path := filepath.Join(dir, strconv.Itoa(n))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := make([]byte, 4)
	binary.LittleEndian.PutUint32(sum, crc32.ChecksumIEEE(data))
	if err := os.WriteFile(path+".0", sum, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestGUI(t *testing.T) (*gui, string) {
	t.Helper()
	root := t.TempDir()
	saves := filepath.Join(root, "SSF4_SaveData")
	os.MkdirAll(saves, 0o755)
	a, err := openArchive(filepath.Join(root, "out"))
	if err != nil {
		t.Fatal(err)
	}
	g := &gui{archive: a, dataDir: root, fresh: map[string]bool{}, log: log.New(io.Discard, "", 0)}
	return g, saves
}

func TestGUIStateListsNewestFirstAndMarksNew(t *testing.T) {
	g, saves := newTestGUI(t)
	base := time.Date(2026, 10, 3, 19, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		writeSlot(t, saves, ringFirst+i, base.Add(time.Duration(i)*time.Minute), byte(i+1))
	}
	g.dirs = []string{saves}
	result, err := g.archive.scan(g.dirs, ringFirst, ringLast)
	if err != nil || len(result.saved) != 3 {
		t.Fatalf("scan saved %d, err %v", len(result.saved), err)
	}
	g.fresh[result.saved[0]] = true

	s := g.state()
	if !s.Found || len(s.Replays) != 3 {
		t.Fatalf("found=%v replays=%d", s.Found, len(s.Replays))
	}
	if !(s.Replays[0].Time > s.Replays[1].Time && s.Replays[1].Time > s.Replays[2].Time) {
		t.Fatalf("not newest first: %+v", s.Replays)
	}
	if s.Replays[0].Time != base.Add(2*time.Minute).UnixMilli() {
		t.Fatalf("time %d, want %d", s.Replays[0].Time, base.Add(2*time.Minute).UnixMilli())
	}
	if !s.Replays[2].New || s.Replays[0].New {
		t.Fatalf("new flags wrong: %+v", s.Replays)
	}
}

func TestGUIWatchInGameRestoresIntoOldestSlot(t *testing.T) {
	g, saves := newTestGUI(t)
	base := time.Date(2026, 10, 3, 19, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		writeSlot(t, saves, ringFirst+i, base.Add(time.Duration(i)*time.Minute), byte(i+1))
	}
	g.dirs = []string{saves}
	g.archive.scan(g.dirs, ringFirst, ringLast)
	oldest := g.state().Replays[9].Name

	// The game cycles on: slot 300 gets a newer match, so the oldest copy
	// is now only in the archive.
	writeSlot(t, saves, ringFirst, base.Add(time.Hour), 0xEE)
	g.archive.scan(g.dirs, ringFirst, ringLast)

	if running, _ := gameRunning(); running {
		t.Skip("SSFIV.exe is running")
	}
	res := g.watchInGame(oldest)
	if !res.OK {
		t.Fatalf("watchInGame: %s", res.Message)
	}
	// Slot 301 held the oldest remaining match, so it is the one replaced.
	got, err := readSlot(saves, ringFirst+1)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join(g.archive.dir, oldest))
	if crc32.ChecksumIEEE(want) != got.crc {
		t.Fatal("slot 301 does not hold the restored replay")
	}
	if res := g.watchInGame(oldest); !res.OK {
		t.Fatalf("second watchInGame: %s", res.Message)
	}
	if res := g.watchInGame(`..\evil.usf4replay`); res.OK {
		t.Fatal("accepted a path outside the archive")
	}
}

func TestGUISetSaveDirValidates(t *testing.T) {
	g, saves := newTestGUI(t)
	if res := g.setSaveDir(filepath.Join(saves, "missing")); res.OK {
		t.Fatal("accepted a missing folder")
	}
	if res := g.setSaveDir(t.TempDir()); res.OK {
		t.Fatal("accepted a folder that is not a save folder")
	}
	os.WriteFile(filepath.Join(saves, "LIST"), []byte{1}, 0o644)
	if res := g.setSaveDir(`"` + saves + `"`); !res.OK {
		t.Fatalf("rejected the save folder: %s", res.Message)
	}
	g.config = guiConfig{}
	g.loadConfig()
	if g.config.SaveDir != saves {
		t.Fatalf("config not saved: %q", g.config.SaveDir)
	}
	if dirs := g.findDirs(); len(dirs) != 1 || dirs[0] != saves {
		t.Fatalf("findDirs %v", dirs)
	}
}
