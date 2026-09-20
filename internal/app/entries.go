package app

import (
	"errors"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

var ErrStale = errors.New("this entry changed since it was loaded")

func (s *Service) Add(e vault.Entry) (vault.Entry, error) {
	e.ID = uuid.Nil()
	return s.write(func(v *vault.Vault) (vault.Entry, error) {
		return v.Put(e)
	})
}

func (s *Service) Update(path string, e vault.Entry) (vault.Entry, error) {
	return s.write(func(v *vault.Vault) (vault.Entry, error) {
		old, err := v.Get(path)
		if err != nil {
			return vault.Entry{}, err
		}
		if !e.Updated.IsZero() && !e.Updated.Equal(old.Updated) {
			return vault.Entry{}, ErrStale
		}
		e.ID, e.Created = old.ID, old.Created
		if e.Path != path {
			if err := v.Delete(path); err != nil {
				return vault.Entry{}, err
			}
		}
		return v.Put(e)
	})
}

func (s *Service) Delete(path string) error {
	_, err := s.write(func(v *vault.Vault) (vault.Entry, error) {
		return vault.Entry{}, v.Delete(path)
	})
	return err
}

func (s *Service) Get(path string) (vault.Entry, error) {
	v, err := s.open()
	if err != nil {
		return vault.Entry{}, err
	}
	defer v.Close()
	return v.Get(path)
}

func (s *Service) List() ([]search.Summary, error) {
	return s.Search("", search.SortPath)
}

func (s *Service) Search(query string, order search.Sort) ([]search.Summary, error) {
	v, err := s.open()
	if err != nil {
		return nil, err
	}
	defer v.Close()
	return search.SearchSort(search.SummarizeAll(v.List()), query, order), nil
}

func (s *Service) open() (*vault.Vault, error) {
	v, err := s.load()
	if err == nil {
		s.afterRead()
	}
	return v, err
}

func (s *Service) load() (*vault.Vault, error) {
	id, err := s.identity()
	if err != nil {
		return nil, err
	}
	return s.loadWith(id)
}

func (s *Service) loadWith(id *crypto.Identity) (*vault.Vault, error) {
	v, err := vault.Load(s.Paths.Vault(), id)
	if err != nil {
		return nil, err
	}
	c := s.CurrentConfig().Vault
	v.SetHistory(c.History, c.MachineName())
	return v, nil
}

func (s *Service) write(fn func(*vault.Vault) (vault.Entry, error)) (vault.Entry, error) {
	id, err := s.identity()
	if err != nil {
		return vault.Entry{}, err
	}
	var e vault.Entry
	err = s.withLock(func(lock *gitsync.Lock) (bool, error) {
		var changed bool
		var err error
		e, changed, err = s.writeLocked(lock, id, fn)
		return changed, err
	})
	if err != nil {
		return vault.Entry{}, err
	}
	return e, nil
}

func (s *Service) writeLocked(lock *gitsync.Lock, id *crypto.Identity, fn func(*vault.Vault) (vault.Entry, error)) (vault.Entry, bool, error) {
	v, err := s.loadWith(id)
	if err != nil {
		return vault.Entry{}, false, err
	}
	defer v.Close()
	e, err := fn(v)
	if err != nil {
		return vault.Entry{}, false, err
	}
	if err := lock.Err(); err != nil {
		return vault.Entry{}, false, err
	}
	if err := v.Save(); err != nil {
		return vault.Entry{}, false, err
	}
	if err := lock.Err(); err != nil {
		return vault.Entry{}, false, err
	}
	changed, err := s.commitOnly(v.Identity())
	return e, changed, err
}
