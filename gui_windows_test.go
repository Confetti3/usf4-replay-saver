//go:build windows

package main

import (
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestGUI(t *testing.T) (*gui, *fakeGame) {
	t.Helper()
	game := newFakeGame(t)
	root := t.TempDir()
	a, err := openArchive(filepath.Join(root, "out"))
	if err != nil {
		t.Fatal(err)
	}
	g := &gui{archive: a, dataDir: root, fresh: map[string]bool{}, log: log.New(io.Discard, "", 0)}
	return g, game
}

func TestGUIStateListsNewestFirstAndMarksNew(t *testing.T) {
	g, game := newTestGUI(t)
	base := time.Date(2026, 10, 3, 19, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		game.match(ringFirst+i, base.Add(time.Duration(i)*time.Minute), byte(i+1))
	}
	g.dirs = []string{game.dir}
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
	for _, r := range s.Replays {
		if !r.CanWatch {
			t.Fatalf("%s cannot be watched", r.Name)
		}
	}
}

func TestGUIWatchInGame(t *testing.T) {
	if running, _ := gameRunning(); running {
		t.Skip("SSFIV.exe is running")
	}
	g, game := newTestGUI(t)
	base := time.Date(2026, 10, 3, 19, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		game.match(ringFirst+i, base.Add(time.Duration(i)*time.Minute), byte(i+1))
	}
	g.dirs = []string{game.dir}
	g.archive.scan(g.dirs, ringFirst, ringLast)
	oldest := g.state().Replays[9].Name
	game.match(ringFirst, base.Add(time.Hour), 0x40)
	g.archive.scan(g.dirs, ringFirst, ringLast)

	if res := g.watchInGame(oldest); !res.OK {
		t.Fatalf("watchInGame: %s", res.Message)
	}
	want, _ := os.ReadFile(filepath.Join(g.archive.dir, oldest))
	if got, err := readSlot(game.dir, ringFirst+1); err != nil || got.crc != crc32.ChecksumIEEE(want) {
		t.Fatal("slot 301 does not hold the restored replay")
	}
	if res := g.watchInGame(`..\evil.usf4replay`); res.OK {
		t.Fatal("accepted a path outside the archive")
	}
}

func TestGUISetSaveDirValidates(t *testing.T) {
	g, game := newTestGUI(t)
	saves := game.dir
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
