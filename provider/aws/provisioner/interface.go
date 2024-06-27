package provisioner

type Storage interface {
	SaveState(id string, data []byte) error
	GetState(id string) ([]byte, error)
	SendStateLog(id, message string) error
}
