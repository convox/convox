package manifest

import (
	"gopkg.in/yaml.v3"
)

func ResolveAnchorAndAlias(data []byte) ([]byte, error) {
	var node interface{}
	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	yamlData, err := yaml.Marshal(&node)
	if err != nil {
		return nil, err
	}
	return yamlData, nil
}
