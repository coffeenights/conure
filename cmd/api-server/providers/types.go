package providers

// ComponentVariables carries plain variables and decrypted secrets for a
// single component, ready to be projected into a ConfigMap and Secret in the
// environment namespace.
type ComponentVariables struct {
	ComponentName string
	Variables     map[string]string
	Secrets       map[string]string
}
