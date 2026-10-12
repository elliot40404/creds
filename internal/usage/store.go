package usage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	maxFileSize = 1 << 20
	lockSuffix  = ".lock"
	magic       = "CUS1"
	headerSize  = len(magic) + 4
	recordSize  = 32
)

type Store struct {
	Path string
	Now  func() time.Time
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) Ranks(id *crypto.Identity) map[uuid.UUID]float64 {
	return s.load(id).Ranks(s.now())
}

func (s *Store) Record(id *crypto.Identity, entry uuid.UUID) error {
	if err := fsutil.EnsureDir(filepath.Dir(s.Path)); err != nil {
		return err
	}
	return fsutil.WithFileLock(s.Path+lockSuffix, func() error {
		l := s.load(id)
		l.Add(entry, s.now())
		return s.write(id, l)
	})
}

func (s *Store) load(id *crypto.Identity) Log {
	data, err := fsutil.ReadFileLimit(s.Path, maxFileSize)
	if err != nil {
		return Log{}
	}
	plain, err := crypto.Open(id, data)
	if err != nil {
		return Log{}
	}
	l, ok := decode(plain)
	if !ok {
		return Log{}
	}
	return l
}

func (s *Store) write(id *crypto.Identity, l Log) error {
	sealed, err := crypto.Seal(id.Recipient(), encode(l))
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.Path, sealed)
}

func encode(l Log) []byte {
	out := make([]byte, 0, headerSize+len(l)*recordSize)
	out = fmt.Appendf(out, "%s%04d", magic, len(l))
	for id, u := range l {
		out = append(out, id[:]...)
		out, _ = binary.Append(out, binary.BigEndian, u.Score)
		out, _ = binary.Append(out, binary.BigEndian, u.Last)
	}
	return out
}

func decode(data []byte) (Log, bool) {
	rest, ok := bytes.CutPrefix(data, []byte(magic))
	if !ok || len(rest) < 4 || strings.Trim(string(rest[:4]), "0123456789") != "" {
		return nil, false
	}
	n, _ := strconv.Atoi(string(rest[:4]))
	rest = rest[4:]
	if n > MaxEntries || n*recordSize > len(rest) || strings.Trim(string(rest[n*recordSize:]), " ") != "" {
		return nil, false
	}
	l := make(Log, n)
	for i := range n {
		r := rest[i*recordSize : (i+1)*recordSize]
		var u Use
		if _, err := binary.Decode(r[16:], binary.BigEndian, &u); err != nil || !u.valid() {
			return nil, false
		}
		l[uuid.UUID(r[:16])] = u
	}
	return l, len(l) == n
}

func (u Use) valid() bool {
	return !math.IsNaN(u.Score) && !math.IsInf(u.Score, 0) && u.Score >= 0 && u.Last >= 0
}
