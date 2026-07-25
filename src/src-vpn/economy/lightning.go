package economy

import "fmt"

// LightningNetwork handles micro-payments for relay/storage nodes.
// This is a stub — real implementation requires lnd or eclair client.
type LightningNetwork struct {
	NodeURL string
}

func NewLightningNetwork(nodeURL string) *LightningNetwork {
	return &LightningNetwork{NodeURL: nodeURL}
}

// CreateInvoice generates a Lightning invoice for node services.
func (l *LightningNetwork) CreateInvoice(amountSat int64, description string) (string, error) {
	// TODO: integrate with lnd REST API
	return fmt.Sprintf("lnbc%dn1pw%stubs", amountSat*1000, description[:4]), nil
}

// PayInvoice pays a Lightning invoice.
func (l *LightningNetwork) PayInvoice(invoice string) error {
	// TODO: integrate with lnd REST API
	return nil
}

// CheckBalance returns node balance in satoshis.
func (l *LightningNetwork) CheckBalance() (int64, error) {
	// TODO: query lnd
	return 0, nil
}
