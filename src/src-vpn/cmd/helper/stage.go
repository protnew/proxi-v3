package main

import "fmt"

// Stage 1 (D8=K) is elevated-spawn. D5/D6 stay unlinked until a dep spike
// passes without breaking the default build (CGO/wintun.dll/admin).
const WFPSublayerName = "PROXI_KILLSWITCH"

func stage1Only() error {
	return fmt.Errorf("stage1 elevated-spawn only; wintun and %s not linked", WFPSublayerName)
}
