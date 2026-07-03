package input

type Context struct {
	actorUID *int
}

func NewContext() Context {
	return Context{}
}

func (context *Context) SetActorUID(actorUID int) {
	context.actorUID = &actorUID
}

func (context *Context) ClearActorUID() {
	context.actorUID = nil
}

func (context Context) ActorUID() (int, bool) {
	if context.actorUID == nil {
		return 0, false
	}
	return *context.actorUID, true
}

