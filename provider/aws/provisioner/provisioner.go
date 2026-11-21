package provisioner

import "errors"

var (
	ErrDeleteInProgress = errors.New("delete in progress")
	ErrGroupDelete      = errors.New("could not delete")
)

type Storage interface {
	SaveState(id string, data []byte, provisioner string, meta map[string]string) error
	GetState(id string) ([]byte, error)
	SendStateLog(id, message string) error
}

type ConnectionInfo struct {
	Host     string
	Port     string
	UserName string
	Password string
	Database string
}
