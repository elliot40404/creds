package fail

import "errors"

type Code int

const (
	OK Code = iota
	General
	Usage
	Locked
	NotFound
	Conflict
	NoVault
)

type Error struct {
	Msg  string
	Hint string
	Code Code
	Err  error
}

func (e *Error) Error() string { return e.Msg }

func (e *Error) Unwrap() error { return e.Err }

type Rule struct {
	Target error
	Code   Code
	Msg    string
	Hint   string
}

func Classify(err error, extra ...Rule) *Error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}
	if r, ok := match(err, extra); ok {
		return r.build(err)
	}
	if r, ok := match(err, rules); ok {
		return r.build(err)
	}
	return &Error{Msg: err.Error(), Code: General, Err: err}
}

func match(err error, list []Rule) (Rule, bool) {
	for _, r := range list {
		if errors.Is(err, r.Target) {
			return r, true
		}
	}
	return Rule{}, false
}

func (r Rule) build(err error) *Error {
	msg := r.Msg
	if msg == "" {
		msg = err.Error()
	}
	return &Error{Msg: msg, Hint: r.Hint, Code: r.Code, Err: err}
}
