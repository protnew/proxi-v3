package main

import (
	"fmt"
	"os"

	"github.com/Microsoft/go-winio"
	"github.com/unkillable-messenger/vpn"
)

func main() {
	conn, err := winio.DialPipe(`\\.\pipe\ProxiHelper`, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	verb := "status"
	if len(os.Args) > 1 {
		verb = os.Args[1]
	}
	var st vpn.HelperStatus
	if verb == "connect" {
		st, err = vpn.ConnectHelper(conn, vpn.HelperConnect{SelfExit: true, Endpoint: "203.0.113.9:443", Token: "live", Npub: "np", Sig: "sg", Exp: 17})
	} else {
		st, err = vpn.QueryHelper(conn)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("state=%s engaged=%v err=%s\n", st.State, st.Engaged, st.Error)
}
