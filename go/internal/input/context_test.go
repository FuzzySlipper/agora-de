package input

import "testing"

func TestContextSetAndClearActorUID(t *testing.T) {
	context := NewContext()
	if _, ok := context.ActorUID(); ok {
		t.Fatal("new input context should not have an actor uid")
	}

	context.SetActorUID(60002)
	actorUID, ok := context.ActorUID()
	if !ok {
		t.Fatal("actor uid was not set")
	}
	if actorUID != 60002 {
		t.Fatalf("actor uid = %d, want 60002", actorUID)
	}

	context.ClearActorUID()
	if _, ok := context.ActorUID(); ok {
		t.Fatal("cleared input context should not have an actor uid")
	}
}

