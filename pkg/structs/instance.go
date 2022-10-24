package structs

import (
	"fmt"
	"os"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

type Instance struct {
	Agent             bool      `json:"agent"`
	Cpu               float64   `json:"cpu"`
	CpuCapacity       float64   `json:"cpu-capacity"`
	CpuAllocatable    float64   `json:"cpu-allocatable"`
	Id                string    `json:"id"`
	Memory            float64   `json:"memory"`
	MemoryCapacity    float64   `json:"memory-capacity"`
	MemoryAllocatable float64   `json:"memory-allocatable"`
	PrivateIp         string    `json:"private-ip"`
	Processes         int       `json:"processes"`
	PublicIp          string    `json:"public-ip"`
	Status            string    `json:"status"`
	Started           time.Time `json:"started"`
}

type Instances []Instance

type InstanceShellOptions struct {
	Command    *string `header:"Command"`
	PrivateKey *string `header:"Private-Key"`
	Height     *int    `header:"Height"`
	Width      *int    `header:"Width"`
}

func (i *Instance) Ip() string {
	if os.Getenv("DEVELOPMENT") == "true" {
		return i.PublicIp
	}

	return i.PrivateIp
}

func (i *Instance) DockerHost() string {
	if h := os.Getenv("TEST_DOCKER_HOST"); h != "" {
		return h
	}

	return fmt.Sprintf("http://%s:2376", i.Ip())
}

func (i *Instance) DockerClient() (*docker.Client, error) {
	return docker.NewClient(i.DockerHost())
}

func (ii Instances) Len() int           { return len(ii) }
func (ii Instances) Less(i, j int) bool { return ii[i].Id < ii[j].Id }
func (ii Instances) Swap(i, j int)      { ii[i], ii[j] = ii[j], ii[i] }

type KeyPair struct {
	Name       *string `json:"name"`
	PrivateKey *string `json:"private-key"`
}
