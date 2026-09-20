package crypto

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
)

const (
	macKeyInfo         = "creds bucket mac v1"
	manifestMACKeyInfo = "creds manifest mac v1"
	macKeySize         = 32
	macMember          = "mac"
)

var errEnvelope = errors.New("bad signed envelope")

func (i *Identity) MACKey() ([]byte, error) {
	return i.macKey(macKeyInfo)
}

func (i *Identity) ManifestMACKey() ([]byte, error) {
	return i.macKey(manifestMACKeyInfo)
}

func (i *Identity) macKey(info string) ([]byte, error) {
	return hkdf.Key(sha256.New, []byte(i.id.String()), nil, info, macKeySize)
}

func Sign(key []byte, name string, payload []byte) ([]byte, error) {
	env := map[string]jsontext.Value{
		macMember: jsontext.Value(strconv.Quote(hex.EncodeToString(mac(key, payload)))),
		name:      payload,
	}
	out, err := json.Marshal(env, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", name, err)
	}
	return out, nil
}

func Verify(key []byte, name string, envelope []byte) ([]byte, error) {
	var env map[string]jsontext.Value
	if err := json.Unmarshal(envelope, &env); err != nil {
		return nil, fmt.Errorf("%w: %w", errEnvelope, err)
	}
	payload, ok := env[name]
	if len(env) != 2 || !ok {
		return nil, fmt.Errorf("%w: want only %s and %s", errEnvelope, macMember, name)
	}
	var sig string
	if err := json.Unmarshal(env[macMember], &sig); err != nil {
		return nil, fmt.Errorf("%w: %w", errEnvelope, err)
	}
	got, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mac(key, payload)) {
		return nil, fmt.Errorf("%w: mac mismatch", errEnvelope)
	}
	return payload, nil
}

func mac(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
