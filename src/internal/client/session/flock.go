package session

type FlockMode uint8

const (
	FlockModeOFD         FlockMode = 0
	FlockModeUnsupported FlockMode = 1
	FlockModeNoop        FlockMode = 2
)
