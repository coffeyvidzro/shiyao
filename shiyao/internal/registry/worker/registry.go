package worker

type Registry struct {
	Consumers ConsumerRegistry
	Jobs      JobRegistry
}
