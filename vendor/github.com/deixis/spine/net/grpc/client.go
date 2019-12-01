package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"io/ioutil"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
	lcontext "github.com/deixis/spine/context"
	"github.com/deixis/spine/log"
	"github.com/deixis/spine/tracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Client is a wrapper for the grpc client.
type Client struct {
	unaryMiddlewares  []UnaryClientMiddleware
	streamMiddlewares []StreamClientMiddleware

	// HTTP is the standard net/http client
	GRPC *grpc.ClientConn
	// PropagateContext tells whether the context should be propagated upstream
	//
	// This should be activated when the upstream endpoint is a LEGO service
	// or another LEGO-compatible service. The context can potentially leak
	// sensitive information, so do not activate it for services that you
	// don't trust.
	PropagateContext bool
}

func NewClient(
	ctx context.Context, target string, opts ...grpc.DialOption,
) (*Client, error) {
	log.FromContext(ctx).Trace("c.grpc.dial", "Dialing...",
		log.String("target", target),
	)
	client := &Client{}

	// Add default dial options
	opts = append(opts,
		grpc.WithUnaryInterceptor(client.unaryInterceptor),
		grpc.WithStreamInterceptor(client.streamInterceptor),
	)

	// Dial GRPC connection
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, err
	}
	client.GRPC = conn
	return client, nil
}

func (c *Client) AppendUnaryMiddleware(m UnaryClientMiddleware) {
	c.unaryMiddlewares = append(c.unaryMiddlewares, m)
}

func (c *Client) AppendStreamMiddleware(m StreamClientMiddleware) {
	c.streamMiddlewares = append(c.streamMiddlewares, m)
}

// WaitForStateReady waits until the connection is ready or the context
// times out
func (c *Client) WaitForStateReady(ctx context.Context) error {
	s := c.GRPC.GetState()
	if s == connectivity.Ready {
		return nil
	}

	log.FromContext(ctx).Trace("c.grpc.wait", "Wait for connection to be ready",
		log.Stringer("state", s),
	)
	if !c.GRPC.WaitForStateChange(ctx, s) {
		// ctx got timeout or canceled.
		return ctx.Err()
	}
	return nil
}

func (c *Client) Close() error {
	return c.GRPC.Close()
}

// WithTLS returns a dial option for the GRPC client that activates
// TLS. This must be used when the server has TLS activated.
func WithTLS(
	certFile, serverNameOverride string,
) (grpc.DialOption, error) {
	creds, err := credentials.NewClientTLSFromFile(certFile, serverNameOverride)
	if err != nil {
		return nil, errors.Wrap(err, "could not load certificate")
	}
	return grpc.WithTransportCredentials(creds), nil
}

// WithMutualTLS returns a dial option for the GRPC client that activates
// a mutual TLS authentication between the server and the client.
func WithMutualTLS(
	serverName, certFile, keyFile, caFile string,
) (grpc.DialOption, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not load client key pair")
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not read ca certificate")
	}
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.Wrap(err, "failed to append ca certs")
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   serverName,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})
	return grpc.WithTransportCredentials(creds), nil
}

// MustDialOption panics if it receives an error
func MustDialOption(opt grpc.DialOption, err error) grpc.DialOption {
	if err != nil {
		panic(err)
	}
	return opt
}

// unaryInterceptor intercepts the execution of a unary RPC on the client.
// invoker is the handler to complete the RPC and it is the responsibility of
// the interceptor to call it. This is an EXPERIMENTAL API.
func (c *Client) unaryInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if c.PropagateContext {
		var err error
		ctx, err = EmbedContext(ctx)
		if err != nil {
			return err
		}

		// Encode shipments
		var shipments []shipment
		lcontext.ShipmentRange(ctx, func(k string, v interface{}) bool {
			shipments = append([]shipment{{k, v}}, shipments...) // prepend
			return true
		})
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&shipments); err != nil {
			return err
		}
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		md[shipmentsMD] = append(md[shipmentsMD], buf.String())
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Build middleware chain and then call it
	next := invoker
	for i := len(c.unaryMiddlewares) - 1; i >= 0; i-- {
		next = c.unaryMiddlewares[i](next)
	}
	return next(ctx, method, req, reply, cc, opts...)
}

type UnaryClientMiddleware func(grpc.UnaryInvoker) grpc.UnaryInvoker

// OpenTracingUnaryClientMiddleware returns a UnaryClientMiddleware that injects
// a child span into the gRPC metadata if a span is found within the given context.
func OpenTracingUnaryClientMiddleware(spanOpts ...opentracing.StartSpanOption) UnaryClientMiddleware {
	return func(next grpc.UnaryInvoker) grpc.UnaryInvoker {
		return func(
			ctx context.Context,
			method string,
			req, reply interface{},
			cc *grpc.ClientConn,
			opts ...grpc.CallOption,
		) error {
			if parent := tracing.SpanFromContext(ctx); parent != nil {
				tracer := tracing.FromContext(ctx)
				span := tracer.StartSpan(
					method,
					append(
						spanOpts,
						opentracing.ChildOf(parent.Context()),
						ext.SpanKindRPCClient,
					)...,
				)
				defer span.Finish()

				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					md = metadata.New(nil)
				} else {
					md = md.Copy()
				}
				format := opentracing.HTTPHeaders
				carrier := metadataReaderWriter{md}
				if err := tracer.Inject(span.Context(), format, carrier); err != nil {
					return err
				}
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			return next(ctx, method, req, reply, cc, opts...)
		}
	}
}

type StreamClientMiddleware func(grpc.Streamer) grpc.Streamer

func (c *Client) streamInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if c.PropagateContext {
		var err error
		ctx, err = EmbedContext(ctx)
		if err != nil {
			return nil, err
		}

		// Encode shipments
		var shipments []shipment
		lcontext.ShipmentRange(ctx, func(k string, v interface{}) bool {
			shipments = append([]shipment{{k, v}}, shipments...) // prepend
			return true
		})
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&shipments); err != nil {
			return nil, err
		}
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		md[shipmentsMD] = append(md[shipmentsMD], buf.String())
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Build middleware chain and then call it
	next := streamer
	for i := len(c.streamMiddlewares) - 1; i >= 0; i-- {
		next = c.streamMiddlewares[i](next)
	}
	return next(ctx, desc, cc, method, opts...)
}

// OpenTracingStreamClientMiddleware returns a StreamClientMiddleware that injects
// a child span into the gRPC metadata if a span is found within the given context.
func OpenTracingStreamClientMiddleware(spanOpts ...opentracing.StartSpanOption) StreamClientMiddleware {
	return func(next grpc.Streamer) grpc.Streamer {
		return func(
			ctx context.Context,
			desc *grpc.StreamDesc,
			cc *grpc.ClientConn,
			method string,
			opts ...grpc.CallOption,
		) (grpc.ClientStream, error) {
			if parent := tracing.SpanFromContext(ctx); parent != nil {
				tracer := tracing.FromContext(ctx)
				span := tracer.StartSpan(
					method,
					append(
						spanOpts,
						opentracing.ChildOf(parent.Context()),
						ext.SpanKindRPCClient,
					)...,
				)
				defer span.Finish()

				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					md = metadata.New(nil)
				} else {
					md = md.Copy()
				}
				format := opentracing.HTTPHeaders
				carrier := metadataReaderWriter{md}
				if err := tracer.Inject(span.Context(), format, carrier); err != nil {
					return nil, err
				}
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			return next(ctx, desc, cc, method, opts...)
		}
	}
}

// metadataReaderWriter satisfies both the opentracing.TextMapReader and
// opentracing.TextMapWriter interfaces.
type metadataReaderWriter struct {
	metadata.MD
}

func (w metadataReaderWriter) Set(key, val string) {
	// The GRPC HPACK implementation rejects any uppercase keys here.
	//
	// As such, since the HTTP_HEADERS format is case-insensitive anyway, we
	// blindly lowercase the key (which is guaranteed to work in the
	// Inject/Extract sense per the OpenTracing spec).
	key = strings.ToLower(key)
	w.MD[key] = append(w.MD[key], val)
}

func (w metadataReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range w.MD {
		for _, v := range vals {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}

	return nil
}
