package manifest

import "strings"

type Timer struct {
	Name        string             `yaml:"-"`
	Annotations ServiceAnnotations `yaml:"annotations,omitempty"`

	Command     string `yaml:"command,omitempty"`
	Schedule    string `yaml:"schedule"`
	Service     string `yaml:"service"`
	Concurrency string `yaml:"concurrency,omitempty"`
}

type Timers []Timer

// skipcq
func (t Timer) AnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range t.Annotations {
		parts := strings.SplitN(a, "=", 2)
		annotations[parts[0]] = parts[1]
	}

	return annotations
}

func (t *Timer) GetName() string {
	return t.Name
}

func (t *Timer) SetName(name string) error {
	t.Name = name
	return nil
}
