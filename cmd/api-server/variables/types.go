package variables

const (
	OrganizationType VariableType = "organization"
	EnvironmentType  VariableType = "environment"
	ComponentType    VariableType = "component"
)

type VariableType string

func (vt VariableType) IsValid() bool {
	return vt == OrganizationType || vt == EnvironmentType || vt == ComponentType
}
