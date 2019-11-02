package do

import "fmt"

func (p *Provider) Resolver() (string, error) {
	return "", fmt.Errorf("no resolver")
}
