package commands

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	k8s "k8s-tui/internal/kubernetes"

	"k8s.io/client-go/rest"
)

// An events table starts with LAST SEEN: the view puts each event's name
// first, so the row's target is it, and its TYPE is where EventType looks.
func TestEventsGetTheirNameFirst(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"kind":"Table","apiVersion":"meta.k8s.io/v1",
		  "columnDefinitions":[{"name":"Last Seen","type":"string"},{"name":"Type","type":"string"},{"name":"Reason","type":"string"},{"name":"Message","type":"string"}],
		  "rows":[{"cells":["5m","Warning","BackOff","Back-off restarting"],
		    "object":{"kind":"PartialObjectMetadata","apiVersion":"meta.k8s.io/v1","metadata":{"name":"web.17f3","namespace":"apps"}}}]}`))
	}))
	defer srv.Close()
	c, _ := k8s.NewClientFromConfig(&rest.Config{Host: srv.URL}, "")
	msg := eventsView().Load(c, false, "apps")().(ItemsLoadedMsg)
	if msg.Err != nil || len(msg.Rows) != 1 || !strings.HasPrefix(msg.Rows[0], "web.17f3   5m   Warning   BackOff") {
		t.Fatalf("rows = %q, err = %v", msg.Rows, msg.Err)
	}
	if s, e, _ := FieldSpan(msg.Rows[0], EventType.Column); msg.Rows[0][s:e] != "Warning" || EventType.Level("Warning") != LevelWarn {
		t.Errorf("type column is %q", msg.Rows[0][s:e])
	}
	if eventsView().RowTarget(msg.Rows[0], "apps") != "apps/web.17f3" {
		t.Errorf("target = %q", eventsView().RowTarget(msg.Rows[0], "apps"))
	}
}
