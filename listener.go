package ln-sdk

import (
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

type Listener interface {
	OnHtlcIntercept(forward *routerrpc.ForwardHtlcInterceptRequest) error
	OnPayment(payment *lnrpc.Payment) error
	OnHtlcEvent(event *routerrpc.HtlcEvent) error
	OnInvoice(invoice *lnrpc.Invoice) error
}
