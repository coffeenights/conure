package timoni

import (
	"github.com/coffeenights/conure/internal/k8s"
	"k8s.io/apimachinery/pkg/runtime"
)

type Values map[string]interface{}

func (t *Values) ExtractFromRawExtension(valuesRaw *runtime.RawExtension) error {
	rawValues, err := k8s.ExtractMapFromRawExtension(valuesRaw)
	if err != nil {
		return err
	}
	*t = rawValues
	return nil
}

func (t *Values) Update(appendValues map[string]interface{}) {
	for key, value := range appendValues {
		(*t)[key] = value
	}
}

func (t *Values) Flag(key string, value bool) {
	(*t)[key] = value
}

func (t *Values) Get() map[string]interface{} {
	return map[string]interface{}{
		"values": t,
	}
}
