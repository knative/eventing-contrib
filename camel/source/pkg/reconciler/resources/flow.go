package resources

import (
	"gopkg.in/yaml.v2"
)

// MarshalCamelFlows marshals a list of flows into their standard yaml representation
func MarshalCamelFlows(flows []map[string]interface{}) (string, error) {
	return marshalCamelFlowStruct(flows)
}

func marshalCamelFlowStruct(flow interface{}) (string, error) {
	b, err := yaml.Marshal(flow)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
