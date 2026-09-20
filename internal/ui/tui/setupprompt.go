package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

type chanPrompter struct {
	out  chan<- tea.Msg
	stop <-chan struct{}
	ctx  context.Context
}

func (c chanPrompter) ask(req *setupReq) (setupAns, error) {
	req.reply = make(chan setupAns, 1)
	if c.ctx.Err() != nil {
		return setupAns{}, errSetupCanceled
	}
	select {
	case c.out <- setupAskMsg{req}:
	case <-c.stop:
		return setupAns{}, errSetupCanceled
	case <-c.ctx.Done():
		return setupAns{}, errSetupCanceled
	}
	select {
	case ans := <-req.reply:
		if ans.canceled {
			return ans, errSetupCanceled
		}
		return ans, nil
	case <-c.stop:
		return setupAns{}, errSetupCanceled
	}
}

func (c chanPrompter) Password(prompt string) (string, error) {
	ans, err := c.ask(&setupReq{kind: reqPassword, prompt: prompt})
	return ans.text, err
}

func (c chanPrompter) Input(prompt, def string) (string, error) {
	ans, err := c.ask(&setupReq{kind: reqInput, prompt: prompt, def: def})
	return ans.text, err
}

func (c chanPrompter) Select(prompt string, options []string) (int, error) {
	ans, err := c.ask(&setupReq{kind: reqSelect, prompt: prompt, options: options})
	return ans.idx, err
}

func (c chanPrompter) SelectFrom(prompt string, options []string, cur int) (int, error) {
	ans, err := c.ask(&setupReq{kind: reqSelect, prompt: prompt, options: options, cur: cur})
	return ans.idx, err
}

func (c chanPrompter) Confirm(prompt string) (bool, error) {
	ans, err := c.ask(&setupReq{kind: reqConfirm, prompt: prompt})
	return ans.ok, err
}

func (c chanPrompter) Show(title, text string) error {
	_, err := c.ask(&setupReq{kind: reqShow, prompt: title, text: text})
	return err
}

func (c chanPrompter) Checks(checks []app.Check) error {
	_, err := c.ask(&setupReq{kind: reqChecks, checks: checks})
	return err
}

func (c chanPrompter) Note(msg string) {
	c.send(setupNoteMsg{msg: msg})
}

func (c chanPrompter) Warn(msg string) {
	c.send(setupNoteMsg{msg: msg, warn: true})
}

func (c chanPrompter) send(msg tea.Msg) {
	select {
	case c.out <- msg:
	case <-c.stop:
	}
}
