package crypto

import (
	"fmt"

	"filippo.io/age"
)

type Identity struct {
	id *age.HybridIdentity
}

type Recipient struct {
	r *age.HybridRecipient
}

func NewIdentity() (*Identity, error) {
	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}
	return &Identity{id: id}, nil
}

func ParseIdentity(s string) (*Identity, error) {
	id, err := age.ParseHybridIdentity(s)
	if err != nil {
		return nil, fmt.Errorf("parse identity: %w", err)
	}
	return &Identity{id: id}, nil
}

func (i *Identity) String() string {
	return i.id.String()
}

func (i *Identity) Recipient() *Recipient {
	return &Recipient{r: i.id.Recipient()}
}

func (r *Recipient) String() string {
	return r.r.String()
}
