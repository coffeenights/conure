package k8s

type Vela struct {
	clientset *GenericClientset
}

func NewVela(clientset *GenericClientset) *Vela {
	return &Vela{clientset: clientset}
}

func (v *Vela) GetTraitDefinition() {

}

func (v *Vela) GetComponentDefinition() {

}
