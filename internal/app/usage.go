package app

import (
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/usage"
)

func (s *Service) usage() *usage.Store {
	return &usage.Store{Path: s.Paths.Usage(), Now: s.Now}
}

func (s *Service) used(id *crypto.Identity, entry uuid.UUID) {
	if s.CurrentConfig().Search.Frecency {
		_ = s.usage().Record(id, entry)
	}
}

func (s *Service) addFrecency(id *crypto.Identity, items []search.Summary) {
	if !s.CurrentConfig().Search.Frecency {
		return
	}
	ranks := s.usage().Ranks(id)
	for i := range items {
		items[i].Frecency = ranks[items[i].ID]
	}
}
