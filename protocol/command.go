package protocol

type Command struct {
	Service string
	Method  string
}

func NewCommand(service, method string) Command {
	return Command{
		Service: service,
		Method:  method,
	}
}
