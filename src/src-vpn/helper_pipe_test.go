package vpn

import (
	"bytes"
	"testing"
)

func TestQueryHelper_ReadsStatus(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("{\"state\":\"off\",\"engaged\":false}\n")
	st, err := QueryHelper(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "off" || st.Engaged {
		t.Fatalf("%+v", st)
	}
}
