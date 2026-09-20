package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	ExportPhrase  = "yes, export plaintext"
	exportFormat  = "creds-export"
	exportVersion = 1
	FormatJSON    = "json"
	FormatCSV     = "csv"
)

var (
	ErrExportFormat = errors.New("unknown export format")
	ErrFileExists   = errors.New("file already exists")
	ErrPhrase       = errors.New("confirmation phrase did not match")
)

const csvNote = "csv leaves out notes, tags and params and cannot be imported back, run creds export --plain --format json for a full backup"

const csvFormulaStart = "=+-@\t\r"

var csvHeader = []string{"path", "type", "username", "host", "url", "field", "value", "secret"}

type exportFile struct {
	Format  string        `json:"format"`
	Version int           `json:"version"`
	Entries []vault.Entry `json:"entries"`
}

func (s *Service) ExportPlain(path, format string, overwrite bool) (int, error) {
	encode, ok := map[string]func([]vault.Entry) ([]byte, error){FormatJSON: encodeJSON, FormatCSV: encodeCSV}[format]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrExportFormat, format)
	}
	if err := checkTarget(path, overwrite); err != nil {
		return 0, err
	}
	if err := s.confirmPhrase(); err != nil {
		return 0, err
	}
	if format == FormatCSV {
		Warn(s.Prompter, csvNote)
	}
	v, err := s.open()
	if err != nil {
		return 0, err
	}
	defer v.Close()
	entries := v.List()
	data, err := encode(entries)
	if err != nil {
		return 0, err
	}
	return len(entries), fsutil.WriteFileAtomic(path, data)
}

func (s *Service) confirmPhrase() error {
	ans, err := s.Prompter.Input(fmt.Sprintf("Type %q to write secrets in plaintext", ExportPhrase), "")
	if err != nil {
		return err
	}
	if ans != ExportPhrase {
		return ErrPhrase
	}
	return nil
}

func checkTarget(path string, overwrite bool) error {
	if overwrite {
		return nil
	}
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s", ErrFileExists, path)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	}
	return err
}

func encodeJSON(entries []vault.Entry) ([]byte, error) {
	return json.Marshal(exportFile{Format: exportFormat, Version: exportVersion, Entries: entries})
}

func encodeCSV(entries []vault.Entry) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, e := range entries {
		if err := w.WriteAll(csvRows(e)); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), w.Error()
}

func csvRows(e vault.Entry) [][]string {
	base := []string{csvCell(e.Path), csvCell(string(e.Type)), csvCell(e.Username), csvCell(e.Host), csvCell(e.URL)}
	if len(e.Fields) == 0 {
		return [][]string{append(base, "", "", "")}
	}
	rows := make([][]string, len(e.Fields))
	for i, f := range e.Fields {
		rows[i] = append(base[:len(base):len(base)], csvCell(f.Name), csvCell(f.Value), strconv.FormatBool(f.Secret))
	}
	return rows
}

func csvCell(v string) string {
	if v != "" && strings.ContainsRune(csvFormulaStart, rune(v[0])) {
		return "'" + v
	}
	return v
}
