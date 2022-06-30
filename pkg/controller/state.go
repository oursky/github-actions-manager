package controller

type State interface {
	Agents() ([]Agent, error)
	GetAgent(id string) (*Agent, error)
	DeleteAgent(id string) error
	UpdateAgent(id string, updater func(*Agent)) error
}
