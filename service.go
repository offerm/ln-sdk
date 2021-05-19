package ln-sdk

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

type Service struct {
	Listener      Listener
	conn          *grpc.ClientConn
	Client        lnrpc.LightningClient
	RouterClient  routerrpc.RouterClient
	InvoiceClient invoicesrpc.InvoicesClient

	HtlcInterceptor routerrpc.Router_HtlcInterceptorClient

	channels []*lnrpc.Channel
}

func (r *Service) Start(listener Listener, ctx *cli.Context) error {
	r.Listener = listener
	r.conn = GetLNDClientConn(ctx)
	r.Client = lnrpc.NewLightningClient(r.conn)
	r.RouterClient = routerrpc.NewRouterClient(r.conn)
	r.InvoiceClient = invoicesrpc.NewInvoicesClient(r.conn)
	if err := r.subscribeInvoices(); err != nil {
		return fmt.Errorf("failed to subscribeInvoices - %v", err)
	}
	if err := r.subscribeHtlcEvents(); err != nil {
		return fmt.Errorf("failed to subscribeHtlcEvents - %v", err)
	}
	if err := r.subscribeChannelEvents(); err != nil {
		return fmt.Errorf("failed to subscribeChannelEvents - %v", err)
	}
	if err := r.setupHtlcInterceptor(); err != nil {
		return fmt.Errorf("failed to setupHtlcInterceptor - %v", err)
	}
	return nil
}

func (r *Service) subscribeHtlcEvents() error {
	ctxt := context.Background()
	events, err := r.RouterClient.SubscribeHtlcEvents(ctxt, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return err
	}
	go func() {
		for {
			event, err := events.Recv()
			if err != nil {
				log.Println("got error from events.Recv()", err)
				return
			}
			r.Listener.OnHtlcEvent(event)
		}
	}()
	return nil
}

func (r *Service) subscribeChannelEvents() error {
	ctxt := context.Background()
	// TODO - activly listen to channels to have a consistent view
	channels, err := r.Client.ListChannels(ctxt, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return err
	}
	r.channels = channels.Channels
	return nil
}

func (r *Service) setupHtlcInterceptor() error {
	ctxt := context.Background()
	var err error
	r.HtlcInterceptor, err = r.RouterClient.HtlcInterceptor(ctxt)
	if err != nil {
		log.Println(err)
		return err
	}

	go func() {
		for {
			forward, err := r.HtlcInterceptor.Recv()
			if err != nil {
				log.Println("got error from HtlcInterceptor.Recv()", err)
				return
			}
			r.Listener.OnHtlcIntercept(forward)
		}
	}()
	return nil
}

func (r *Service) subscribeInvoices() error {
	ctxt := context.Background()
	invoices, err := r.Client.SubscribeInvoices(ctxt, &lnrpc.InvoiceSubscription{
		//AddIndex: 1,
	})
	if err != nil {
		return err
	}
	go func() {
		for {
			invoice, err := invoices.Recv()
			if err != nil {
				log.Println("got error from invoices.Recv()", err)
				return
			}
			r.Listener.OnInvoice(invoice)
		}
	}()
	return nil
}

func (r *Service) SubscribeSingleInvoice(req *invoicesrpc.SubscribeSingleInvoiceRequest) error {
	ctxt := context.Background()
	singleInvoiceClient, err := r.InvoiceClient.SubscribeSingleInvoice(ctxt, req)
	if err != nil {
		return fmt.Errorf("can't SubscribeSingleInvoice for request %v - %v", req, err)
	}
	go func() {
		for {
			singleInvoiceUpdate, err := singleInvoiceClient.Recv()
			if err != nil {
				log.Println("got error from singleInvoiceClient.Recv()", err)
				return
			}
			r.Listener.OnInvoice(singleInvoiceUpdate)
		}
	}()
	return nil
}

func (r *Service) Cleanup() {
	r.conn.Close()
}

func (r *Service) OnInvoice(invoice *lnrpc.Invoice) error {
	log.Println("OnInvoice called", hex.EncodeToString(invoice.RHash), invoice.ValueMsat,
		hex.EncodeToString(invoice.PaymentAddr), lnrpc.Invoice_InvoiceState_name[int32(invoice.State)],
		invoice.IsKeysend)
	return nil
}

func (r *Service) OnHtlcEvent(event *routerrpc.HtlcEvent) error {
	log.Println("OnHtlcEvent called ", event.IncomingChannelId, event.OutgoingChannelId,
		routerrpc.HtlcEvent_EventType_name[int32(event.EventType)], event.Event)
	return nil
}

func (r *Service) OnPayment(payment *lnrpc.Payment) error {
	log.Println("OnPayment called", payment.PaymentHash, payment.ValueMsat,
		lnrpc.Payment_PaymentStatus_name[int32(payment.Status)])
	return nil
}

func (r *Service) OnHtlcIntercept(forward *routerrpc.ForwardHtlcInterceptRequest) error {
	log.Println("OnHtlcIntercept called", hex.EncodeToString(forward.PaymentHash), forward.OutgoingRequestedChanId,
		forward.IncomingAmountMsat, forward.OutgoingAmountMsat)
	log.Println("default habdling - Resume")
	err := r.HtlcInterceptor.Send(&routerrpc.ForwardHtlcInterceptResponse{
		IncomingCircuitKey: forward.IncomingCircuitKey,
		Action:             routerrpc.ResolveHoldForwardAction_RESUME,
	})
	if err != nil {
		log.Println("failed to resume forward htlc ", err)
	}
	return nil
}

func (r *Service) SendPaymentKeySend(destination []byte, customRecords map[uint64][]byte, onChanId uint64) error {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal(err)
	}
	hash := sha256.Sum256(secret)
	customRecords[KeySendSecret] = secret
	features := make([]lnrpc.FeatureBit, 1)
	features[0] = lnrpc.FeatureBit_TLV_ONION_OPT
	log.Println("making a keysend payment 1 sat", "hash", hex.EncodeToString(hash[:]), "secret", hex.EncodeToString(secret))

	// TODO- work with time out. This should end in 5 seconds
	ctxt := context.Background()
	paymentClient, err := r.RouterClient.SendPaymentV2(ctxt, &routerrpc.SendPaymentRequest{
		Dest:         destination,
		AmtMsat:      1000,
		PaymentHash:  hash[:],
		FeeLimitMsat: 0,
		//OutgoingChanId: 	  onChanId
		OutgoingChanIds:   []uint64{onChanId},
		DestCustomRecords: customRecords,
		DestFeatures:      features,
		MaxParts:          0,
		NoInflightUpdates: false,
		TimeoutSeconds:    10,
		FinalCltvDelta:    40 + 3,
	})
	if err != nil {
		return fmt.Errorf("got error from RouterClient.SendPaymentV2 - %v", err)
	}

	go func() {
		for {
			payment, err := paymentClient.Recv()
			if err != nil {
				log.Println(err)
				return
			}
			r.OnPayment(payment)
		}
	}()

	return nil
}

func (r Service) PubKeyForChanId(chanId uint64) (string, error) {
	for _, channel := range r.channels {
		if channel.ChanId == chanId {
			return channel.RemotePubkey, nil
		}
	}
	return "", fmt.Errorf("can't find a channel with channel ID of %v", chanId)
}

func (r *Service) MakeHashPaymentAndMonitor(destination []byte, chanId uint64, hash []byte, addr []byte, amtMsat uint64) error {
	ctxt := context.Background()
	paymentClient, err := r.RouterClient.SendPaymentV2(ctxt, &routerrpc.SendPaymentRequest{
		Dest: destination,
		// TODO - pass fee as parameter
		AmtMsat:           int64(amtMsat), //11000,
		PaymentHash:       hash,
		FeeLimitMsat:      0,
		OutgoingChanIds:   []uint64{chanId},
		MaxParts:          0,
		NoInflightUpdates: false,
		TimeoutSeconds:    10,
		FinalCltvDelta:    40 + 3,
		PaymentAddr:       addr,
	})
	if err != nil {
		return err
	}
	go func() {
		for {
			payment, err := paymentClient.Recv()
			if err != nil {
				log.Println(err)
				return
			}
			r.Listener.OnPayment(payment)
		}
	}()
	return nil

}

// generic code - move to service. check if we are getting informed about hold invoices when staring with subscribeinvoice
func (r *Service) cancelOneSidePendingHoldInvoices() {
	hashesToCancel := make([][]byte, 0)
	ctxt := context.Background()
	resp, err := r.Client.ListChannels(ctxt, &lnrpc.ListChannelsRequest{})
	if err != nil {
		panic(err)
	}
	for _, channel := range resp.Channels {
		for _, pendingHtlc := range channel.PendingHtlcs {
			if pendingHtlc.Incoming {
				hashesToCancel = append(hashesToCancel, pendingHtlc.HashLock)
			}
		}
	}
	for _, hash := range hashesToCancel {
		log.WithField("hash", hex.EncodeToString(hash)).Info("Canceling one side payment")
		_, err = r.InvoiceClient.CancelInvoice(ctxt, &invoicesrpc.CancelInvoiceMsg{
			PaymentHash: hash,
		})
		if err != nil {
			panic(err)
		}

	}
	//hashString:="298acb8771c127ab3290b3ab5c45fef820c50aef8ff3f56c48addb7b75266d5c"
	//hash,err := hex.DecodeString(hashString)
	//if err!=nil{
	//	panic(err)
	//}

}

// generic move to service
func (r *Service) NewHoldPayReq(rebalanceHash []byte, amtMsat uint64) ([]byte, error) {
	ctxt := context.Background()
	resp, err := r.InvoiceClient.AddHoldInvoice(ctxt, &invoicesrpc.AddHoldInvoiceRequest{
		Hash:      rebalanceHash,
		ValueMsat: int64(amtMsat),
	})
	if err != nil {
		log.WithField("error", err).Error("failed tp create rebalance hold invoice")
		return nil, err

	}
	payReqBytes := []byte(resp.PaymentRequest)
	r.SubscribeSingleInvoice(&invoicesrpc.SubscribeSingleInvoiceRequest{
		RHash: rebalanceHash,
	})
	return payReqBytes, nil
}
