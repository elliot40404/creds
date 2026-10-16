package app

import (
	"context"

	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/update"
)

func (s *Service) CheckUpdate(ctx context.Context, c update.Checker) (update.Release, error) {
	if _, ok := update.Parse(c.Version); !ok {
		return update.Release{}, nil
	}
	claimed, err := s.claimUpdateCheck()
	if err != nil || !claimed {
		return update.Release{}, err
	}
	rel, err := c.Latest(ctx)
	if err != nil {
		return update.Release{}, err
	}
	_, err = gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		st.UpdateLatest, st.UpdateURL = rel.Tag, rel.URL
		return true, nil
	})
	return rel, err
}

func (s *Service) claimUpdateCheck() (bool, error) {
	now := s.now()
	claimed := false
	_, err := gitsync.UpdateState(s.Paths.State(), func(st *gitsync.State) (bool, error) {
		claimed = update.Due(st.UpdateChecked, now)
		if claimed {
			st.UpdateChecked = now.UTC()
		}
		return claimed, nil
	})
	return claimed, err
}
