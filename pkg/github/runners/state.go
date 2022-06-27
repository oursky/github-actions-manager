package runners

type Instance struct {
	ID       int64
	Name     string
	IsOnline bool
	IsBusy   bool
	Labels   []string
}

type State struct {
	Epoch     int64
	Instances map[string]Instance
}

func (s *State) Lookup(name string, id int64) (*Instance, bool) {
	if name == "" {
		return nil, false
	}
	if inst, ok := s.Instances[name]; ok {
		// id == 0 -> unknown, don't check ID
		if id == 0 || inst.ID == id {
			return &inst, true
		}
	}
	return nil, false
}
