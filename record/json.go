package record

import (
	"encoding/json"

	"foundry/domain"
)

// encode renders act as human-readable, stable JSON for durable storage.
func encode(act *domain.Act) ([]byte, error) {
	return json.MarshalIndent(act, "", "  ")
}

// decode parses the JSON produced by encode back into an Act.
func decode(data []byte) (*domain.Act, error) {
	act := &domain.Act{}
	if err := json.Unmarshal(data, act); err != nil {
		return nil, err
	}
	return act, nil
}
