package models

import "encoding/json"

// Text is a string an answer has only when it was asked for, even when it is empty.
type Text struct {
	Value string
	Set   bool
}

func TextOf(value string) Text {
	return Text{Value: value, Set: true}
}

func (t Text) IsZero() bool { return !t.Set }

func (t Text) MarshalText() ([]byte, error) { return []byte(t.Value), nil }

func (t Text) MarshalJSON() ([]byte, error) { return json.Marshal(t.Value) }

func (t *Text) UnmarshalJSON(data []byte) error {
	t.Set = true
	return json.Unmarshal(data, &t.Value)
}
