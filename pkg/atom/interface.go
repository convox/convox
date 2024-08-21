package atom

type Interface interface {
	Apply(namespace, name string, cfg *ApplyConfig) error
	Cancel(ns, name string) error
	Status(ns, name string) (string, string, error)
	// Wait(ns, name string) error
}
