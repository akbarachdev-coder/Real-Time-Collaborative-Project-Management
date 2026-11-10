package utils

import (
	"encoding/json"
	"time"
)

// NullUint distinguishes an absent JSON field (no change) from an explicit
// null (clear the value).
type NullUint struct {
	Value uint
	Valid bool
}

func (n *NullUint) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	var v uint
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.Value = v
	n.Valid = true
	return nil
}

type NullTime struct {
	Value time.Time
	Valid bool
}

func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	n.Value = t
	n.Valid = true
	return nil
}
