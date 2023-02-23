package grpc

import (
	"context"

	scontext "github.com/deixis/spine/context"
	"github.com/deixis/spine/log"
	"google.golang.org/grpc/metadata"
)

const (
	transitMD   = "context-transit-bin"
	shipmentsMD = "context-shipments-bin"
)

// ExtractTransit extracts transit from ctx or creates a new one
func ExtractTransit(ctx context.Context) (context.Context, error) {
	var tr scontext.Transit
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		ctx, tr = scontext.NewTransitWithContext(ctx)
		log.Trace(ctx, "grpc.transit.new", "New transit", log.String("id", tr.UUID()))
		return ctx, nil
	}

	// Transit
	if data, ok := md[transitMD]; ok {
		tr := scontext.TransitFactory()
		if err := tr.UnmarshalBinary([]byte(data[0])); err != nil {
			return nil, err
		}
		ctx = scontext.TransitWithContext(ctx, tr)
		log.Trace(ctx, "grpc.transit.extract", "Extract transit", log.String("uuid", tr.UUID()))
	} else {
		ctx, tr = scontext.NewTransitWithContext(ctx)
		log.Trace(ctx, "grpc.transit.new", "New transit", log.String("uuid", tr.UUID()))
	}
	return ctx, nil
}

func EmbedContext(ctx context.Context) (context.Context, error) {
	md := metadata.MD{}
	// Transit
	tr := scontext.TransitFromContext(ctx)
	if tr != nil {
		data, err := tr.Transmit().MarshalBinary()
		if err != nil {
			return nil, err
		}
		md[transitMD] = append(md[transitMD], string(data))
	}

	return metadata.NewOutgoingContext(ctx, md), nil
}

type shipment struct {
	Key   string
	Value interface{}
}
