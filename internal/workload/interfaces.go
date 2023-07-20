package workload

type TypeInterface interface {
	Build() error
	Spawn() error
}
