package variables

import "errors"

var (
	ErrVariableTypeRequiresApplicationID = errors.New("this type requires application_id and environment_id")
	ErrVariableTypeRequiresComponentID   = errors.New(
		"this type requires application_id, environment_id and component_id")
	ErrVariableTypeNotValid  = errors.New("type is not valid")
	ErrVariableAlreadyExists = errors.New("variable already exists")
	ErrVariableNameNotValid  = errors.New("variable name is not valid")
)
