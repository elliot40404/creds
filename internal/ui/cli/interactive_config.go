package cli

import (
	"github.com/elliot40404/creds/internal/app"
)

const configActionEdit = "edit the file in my editor"

func askConfigKey(iv *interview) error {
	return iv.arg(0, func() (string, error) {
		s, err := iv.service()
		if err != nil {
			return "", err
		}
		return iv.chooseKey(s)
	})
}

func (iv *interview) chooseKey(s *app.Service) (string, error) {
	fields := s.ConfigFields()
	options := make([]string, 0, len(fields))
	for _, f := range fields {
		options = append(options, f.Key+" = "+f.Value)
	}
	i, err := iv.p.Select("Setting", options)
	if err != nil {
		return "", err
	}
	return fields[i].Key, nil
}

func askConfigSet(iv *interview) error {
	if err := askConfigKey(iv); err != nil || len(iv.args) == 0 {
		return err
	}
	s, err := iv.service()
	if err != nil {
		return err
	}
	current, err := s.ConfigGet(iv.args[0])
	if err != nil {
		return err
	}
	for _, f := range s.ConfigFields() {
		if f.Key == iv.args[0] && len(f.Choices) > 0 {
			return iv.arg(1, func() (string, error) { return iv.choose("New value", f.Choices) })
		}
	}
	return iv.arg(1, iv.input("New value", current))
}

func askConfig(iv *interview) error {
	s, err := iv.service()
	if err != nil {
		return err
	}
	action, err := iv.choose("Settings", []string{"change one setting", configActionEdit})
	if err != nil {
		return err
	}
	if action == configActionEdit {
		return iv.switchTo([]string{"config", "edit"})
	}
	key, err := iv.chooseKey(s)
	if err != nil || key == "" {
		return err
	}
	return iv.switchTo([]string{"config", "set"}, key)
}
