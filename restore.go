package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var errGameRunning = errors.New("close Street Fighter IV first; the game rewrites its save files while it runs")

// restore puts an archived replay back into the game's recent-match ring so
// the game can play it. It replaces the oldest ring slot, after making sure
// that slot is archived first. The game must be closed. It returns the slot
// written, or -1 when the replay was already in the ring.
func restore(file string, dir string, a *archive) (int, error) {
	if running, err := gameRunning(); err == nil && running {
		return 0, errGameRunning
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	if err := validateReplay(data); err != nil {
		return 0, err
	}
	crc := crc32.ChecksumIEEE(data)

	target, err := oldestRingSlot(dir, crc)
	if err != nil || target < 0 {
		return target, err
	}
	if s, err := readSlot(dir, target); err == nil {
		if _, _, err := a.save(s); err != nil {
			return 0, fmt.Errorf("could not archive slot %d before replacing it: %w", target, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) && err != errNotReplay {
		return 0, fmt.Errorf("slot %d could not be read, so it was left alone: %w", target, err)
	}

	path := filepath.Join(dir, strconv.Itoa(target))
	if err := writeReplaced(path, data); err != nil {
		return 0, err
	}
	sum := make([]byte, 4)
	binary.LittleEndian.PutUint32(sum, crc)
	if err := writeReplaced(path+".0", sum); err != nil {
		return 0, err
	}
	return target, nil
}

// oldestRingSlot picks the ring slot to replace: an empty or unreadable slot
// first, otherwise the one with the oldest game timestamp. It returns -1 when
// the replay is already in the ring.
func oldestRingSlot(dir string, crc uint32) (int, error) {
	best := -1
	var bestTime time.Time
	for n := ringFirst; n <= ringLast; n++ {
		s, err := readSlot(dir, n)
		if err == errChecksum {
			return 0, fmt.Errorf("slot %d is mid-write or damaged; start and quit the game once, then try again", n)
		}
		if err != nil {
			if best < 0 || !bestTime.IsZero() {
				best, bestTime = n, time.Time{}
			}
			continue
		}
		if s.crc == crc {
			return -1, nil
		}
		if best < 0 || (!bestTime.IsZero() && s.written.Before(bestTime)) {
			best, bestTime = n, s.written
		}
	}
	return best, nil
}

func writeReplaced(path string, data []byte) error {
	tmp := path + ".restore-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
