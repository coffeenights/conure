package timoni

import (
	"encoding/json"
	"strings"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	timoniinternal "github.com/coffeenights/conure/internal/timoni"
)

// timoniValues mirrors the values-coercion logic that the component handler
// previously inlined: marshal the Component's Spec.Values RawExtension to
// JSON, decode with UseNumber so integers stay integers (the default decoder
// would float them and break CUE typing), and wrap under the "values" key
// that Timoni modules expect.
func timoniValues(comp *conurev1alpha1.Component) (map[string]interface{}, error) {
	valuesJSON, err := json.Marshal(comp.Spec.Values)
	if err != nil {
		return nil, err
	}
	values := timoniinternal.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	d.UseNumber()
	if err := d.Decode(&values); err != nil {
		return nil, err
	}
	return values.Get(), nil
}
