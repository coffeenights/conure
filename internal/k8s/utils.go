package k8s

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
)

// ExtractMapFromRawExtension extracts a map from a k8s RawExtension
func ExtractMapFromRawExtension(data *runtime.RawExtension) (map[string]interface{}, error) {
	var result map[string]interface{}
	bytesData, err := data.MarshalJSON()
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytesData, &result)
	if err != nil {
		panic(err)
	}
	return result, err
}

// ExtractValuesFromRawExtension extracts values used in timoni templates from a RawExtension and return a map of values with the correct format
func ExtractValuesFromRawExtension(valuesRaw *runtime.RawExtension) (map[string]interface{}, error) {
	rawValues, err := ExtractMapFromRawExtension(valuesRaw)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"values": rawValues,
	}, nil
}
