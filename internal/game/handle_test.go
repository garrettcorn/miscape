package game

import (
	"testing"

	"lanternfall/internal/protocol"
)

// Adding a message type to the protocol without a game handler (or the other
// way round) would silently drop it; this keeps the two in step.
func TestEveryMessageHasAHandler(t *testing.T) {
	for typ, needsRid := range protocol.Types() {
		if typ == "hello" {
			continue // handled by Join
		}
		_, req := requestHandlers[typ]
		_, stream := streamHandlers[typ]
		switch {
		case !req && !stream:
			t.Errorf("message %q has no handler", typ)
		case req && !needsRid:
			t.Errorf("request %q must carry a rid (acks are cached by rid)", typ)
		case stream && needsRid:
			t.Errorf("stream message %q is declared with a rid but never acked", typ)
		}
	}
	for typ := range requestHandlers {
		if _, ok := protocol.Types()[typ]; !ok {
			t.Errorf("handler %q isn't a protocol message", typ)
		}
	}
	for typ := range streamHandlers {
		if _, ok := protocol.Types()[typ]; !ok {
			t.Errorf("handler %q isn't a protocol message", typ)
		}
	}
}
