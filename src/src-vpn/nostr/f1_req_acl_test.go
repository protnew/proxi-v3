package nostr

import (
	"encoding/json"
	"testing"
)

func TestF1_EmptyREQHidesForeignPrivate(t *testing.T) {
	r := NewRelay(100, nil)
	r.mu.Lock()
	r.events = []Event{
		{ID: "pub", PubKey: "authorA", Kind: 1, Content: "public", CreatedAt: 1},
		{ID: "dm", PubKey: "authorA", Kind: 4, Content: "secret-dm", Tags: [][]string{{"p", "bob"}}, CreatedAt: 2},
		{ID: "inv", PubKey: "authorA", Kind: 30090, Content: `{"wtAddr":"1.2.3.4:4433"}`, Tags: [][]string{{"p", "bob"}}, CreatedAt: 3},
		{ID: "mine", PubKey: "carol", Kind: 30090, Content: "own", CreatedAt: 4},
	}
	r.mu.Unlock()

	conn := newMockConn()
	carol := &Client{conn: conn, subscriptions: map[string]*Subscription{}, AuthPubkey: "carol"}
	r.handleReq(carol, []json.RawMessage{json.RawMessage(`"s"`), json.RawMessage(`{}`)})
	nForeign := 0
	for _, w := range conn.getWritten() {
		msg, ok := w.([]interface{})
		if !ok || len(msg) < 3 || msg[0] != "EVENT" {
			continue
		}
		ev, ok := msg[2].(Event)
		if !ok {
			t.Fatalf("event type %T", msg[2])
		}
		if ev.ID == "dm" || ev.ID == "inv" {
			nForeign++
		}
	}
	if nForeign != 0 {
		t.Fatalf("foreign JWT saw %d private events", nForeign)
	}
}

func TestF1_EmptyREQNeedsAuth(t *testing.T) {
	r := NewRelay(10, nil)
	conn := newMockConn()
	anon := &Client{conn: conn, subscriptions: map[string]*Subscription{}}
	r.handleReq(anon, []json.RawMessage{json.RawMessage(`"s"`), json.RawMessage(`{}`)})
	if _, ok := anon.subscriptions["s"]; ok {
		t.Fatal("anonymous REQ {} must not subscribe")
	}
}

func TestF1_HashPParsed(t *testing.T) {
	var f Filter
	if err := json.Unmarshal([]byte(`{"#p":["Bb"],"kinds":[1]}`), &f); err != nil {
		t.Fatal(err)
	}
	if len(f.TagFilters["p"]) != 1 || f.TagFilters["p"][0] != "Bb" {
		t.Fatalf("tag filters %#v", f.TagFilters)
	}
	ev := &Event{Kind: 1, Tags: [][]string{{"p", "bb"}}}
	if !matchFilter(ev, &f) {
		t.Fatal("expected #p match case-insensitive")
	}
}
