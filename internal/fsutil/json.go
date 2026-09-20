package fsutil

import "encoding/json/v2"

func ReadJSON(path string, limit int64, v any) error {
	data, err := ReadFileLimit(path, limit)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v, json.RejectUnknownMembers(true))
}

func WriteJSON(path string, v any) error {
	data, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data)
}
