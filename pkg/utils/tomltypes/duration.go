package tomltypes

import "time"

type Duration struct{ time.Duration }

func (s *Duration) UnmarshalText(text []byte) error {
	var err error
	s.Duration, err = time.ParseDuration(string(text))
	return err
}

func (s *Duration) Value() *time.Duration {
	if s == nil {
		return nil
	}
	return &s.Duration
}
