package provisioner

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
