package manifest

type Timer struct {
	Name        string             `yaml:"-"`
	Annotations ServiceAnnotations `yaml:"annotations,omitempty"`

	Command     string `yaml:"command"`
	Schedule    string `yaml:"schedule"`
	Service     string `yaml:"service"`
	Concurrency string `yaml:"concurrency"`
}

type Timers []Timer

// skipcq
func (t Timer) AnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range t.Annotations {
		for k, v := range a {
			annotations[k] = v.(string)
		}
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
