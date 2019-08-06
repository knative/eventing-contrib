package resources

import (
	"fmt"
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

// AddSinkToCamelFlow adds the knative endpoint sink to a user-provided yaml flow
func AddSinkToCamelFlow(flow map[interface{}]interface{}, sink string) map[interface{}]interface{} {
	var from map[interface{}]interface{}
	var steps []interface{}
	if from1, ok := flow["from"]; ok && reflect.TypeOf(from1).Kind() == reflect.Map {
		from = from1.(map[interface{}]interface{})
	} else {
		from = make(map[interface{}]interface{})
	}
	if steps1, ok := from["steps"]; ok && reflect.TypeOf(steps1).Kind() == reflect.Slice {
		steps = steps1.([]interface{})
	} else {
		steps = make([]interface{}, 0)
	}

	to := make(map[interface{}]interface{})
	to["uri"] = fmt.Sprintf("knative://endpoint/%s", sink)
	step := make(map[interface{}]interface{})
	step["to"] = to

	steps = append(steps, step)
	from["steps"] = steps
	flow["from"] = from
	return flow
}

// UnmarshalCamelFlow unmarshals a flow from its standard yaml representation
func UnmarshalCamelFlow(data string) (map[interface{}]interface{}, error) {
	f := make(map[interface{}]interface{})
	err := yaml.Unmarshal([]byte(data), &f)
	return f, err
}

// MarshalCamelFlow marshals a flow into its standard yaml representation
func MarshalCamelFlow(flow map[interface{}]interface{}) (string, error) {
	return marshalCamelFlowStruct(flow)
}

// MarshalCamelFlows marshals a list of flows into their standard yaml representation
func MarshalCamelFlows(flows []map[interface{}]interface{}) (string, error) {
	return marshalCamelFlowStruct(flows)
}

func marshalCamelFlowStruct(flow interface{}) (string, error) {
	b, err := yaml.Marshal(flow)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CopyCamelFlow returns a copy of the flow obtained through marshal and unmarshal into the standard yaml representation
func CopyCamelFlow(flow interface{}) (map[interface{}]interface{}, error) {
	m, err := marshalCamelFlowStruct(flow)
	if err != nil {
		return nil, err
	}
	return UnmarshalCamelFlow(m)
}
