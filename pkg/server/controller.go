package server

type Controller struct {
	config *Configuration
}

func NewController(config *Configuration) *Controller {
	controller := &Controller{
		config: config,
	}

	return controller
}
