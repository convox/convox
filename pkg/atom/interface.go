package atom

type Interface interface {
	Apply(ns, name, release string, template []byte, timeout int32) error
	Cancel(ns, name string) error
	Status(ns, name string) (string, string, error)
	// Wait(ns, name string) error
}
