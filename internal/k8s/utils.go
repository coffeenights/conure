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
