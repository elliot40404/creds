package fail

import (
	"encoding/json/v2"
	"strings"

	"github.com/elliot40404/creds/internal/safetext"
)

type payload struct {
	Error string `json:"error"`
	Hint  string `json:"hint"`
	Code  Code   `json:"code"`
}

func JSON(e *Error) []byte {
	b, err := json.Marshal(payload{Error: safetext.Text(e.Msg), Hint: safetext.Text(e.Hint), Code: e.Code})
	if err != nil {
		return []byte(`{"error":"internal error","hint":"","code":1}`)
	}
	return b
}

func Text(e *Error) string {
	var b strings.Builder
	b.WriteString("error: " + safetext.Text(e.Msg) + "\n")
	if e.Hint != "" {
		b.WriteString("fix: " + safetext.Text(e.Hint) + "\n")
	}
	return b.String()
}
