package grpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/status"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	olog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/deixis/spine/bg"
	"github.com/deixis/spine/cache"
	"github.com/deixis/spine/config"
	lcontext "github.com/deixis/spine/context"
	"github.com/deixis/spine/disco"
	"github.com/deixis/spine/log"
	lnet "github.com/deixis/spine/net"
	"github.com/deixis/spine/schedule"
	"github.com/deixis/spine/stats"
	"github.com/deixis/spine/tracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

// A Server defines parameters for running a lego compatible GRPC server
type Server struct {
	mode uint32
	addr string

	opts              []grpc.ServerOption
	registrations     []registration
	services          []service
	unaryMiddlewares  []UnaryServerMiddleware
	streamMiddlewares []StreamServerMiddleware

	creds grpc.ServerOption

	ctx    context.Context
	log    log.Logger
	config *config.Config

	GRPC *grpc.Server
}

// NewServer creates a new GRPC server
func NewServer() *Server {
	return &Server{
		unaryMiddlewares: []UnaryServerMiddleware{
			mwUnaryServerTracing,
			mwUnaryServerLogging,
			mwUnaryServerStats,
		},
		streamMiddlewares: []StreamServerMiddleware{
			mwStreamServerTracing,
			mwStreamServerLogging,
			mwStreamServerStats,
		},
	}
}

// Handle just injects the GRPC server to register a service. The function
// is called back only when Serve is called. This must be called before
// invoking Serve.
func (s *Server) Handle(f func(*grpc.Server)) {
	s.registrations = append(s.registrations, f)
}

// RegisterService register a service and its implementation to the gRPC
// server. Called from the IDL generated code.
//
// The function is called back only when Serve is called. This must be called
// before invoking Serve.
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	s.services = append(s.services, service{sd: sd, ss: ss})
}

// SetOptions changes the handler options
func (s *Server) SetOptions(opts ...grpc.ServerOption) {
	s.opts = append(s.opts, opts...)
}

// AppendUnaryMiddleware appends an unary middleware to the call chain
func (s *Server) AppendUnaryMiddleware(m UnaryServerMiddleware) {
	s.unaryMiddlewares = append(s.unaryMiddlewares, m)
}

// AppendStreamMiddleware appends a stream middleware to the call chain
func (s *Server) AppendStreamMiddleware(m StreamServerMiddleware) {
	s.streamMiddlewares = append(s.streamMiddlewares, m)
}

// ActivateTLS activates TLS on this handler. That means only incoming TLS
// connections are allowed.
//
// If the certificate is signed by a certificate authority, the certFile should
// be the concatenation of the server's certificate, any intermediates,
// and the CA's certificate.
//
// Clients are not authenticated.
func (s *Server) ActivateTLS(certFile, keyFile string) {
	// Create the TLS credentials
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		panic(err)
	}
	s.creds = grpc.Creds(creds)
}

// ActivateMutualTLS activates TLS on this handler. That means only incoming TLS
// connections are allowed and clients must authenticate themselves to the
// server.
//
// If the certificate is signed by a certificate authority, the certFile should
// be the concatenation of the server's certificate, any intermediates,
// and the CA's certificate.
func (s *Server) ActivateMutualTLS(certFile, keyFile, caFile string) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(errors.Wrap(err, "could not load server key pair"))
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caFile)
	if err != nil {
		panic(errors.Wrap(err, "could not read ca certificate"))
	}
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		panic(errors.Wrap(err, "failed to append client certs"))
	}

	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	})
	s.creds = grpc.Creds(creds)
}

// Serve starts serving HTTP requests (blocking call)
func (s *Server) Serve(ctx context.Context, addr string) error {
	s.ctx = ctx
	s.log = log.FromContext(ctx)

	cfg := config.Config{}
	if err := config.TreeFromContext(ctx).Unmarshal(&cfg); err != nil {
		return err
	}
	s.config = &cfg

	defer atomic.StoreUint32(&s.mode, lnet.StateDown)

	// Register interceptor
	s.opts = append(
		s.opts,
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	)

	tlsEnabled := s.creds != nil
	if tlsEnabled {
		s.SetOptions(s.creds)
	}

	s.GRPC = grpc.NewServer(s.opts...)
	s.addr = addr

	// Register endpoints/services
	for _, registration := range s.registrations {
		registration(s.GRPC)
	}
	for _, service := range s.services {
		s.GRPC.RegisterService(service.sd, service.ss)
	}

	// Register reflection service on gRPC server
	reflection.Register(s.GRPC)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.log.Trace("s.grpc.listen", "Listening...",
		log.String("addr", addr),
		log.Bool("tls", tlsEnabled),
	)
	atomic.StoreUint32(&s.mode, lnet.StateUp)
	err = s.GRPC.Serve(lis)
	switch err := err.(type) {
	case *net.OpError:
		if err.Op == "accept" && s.isDraining() {
			return nil
		}
	}
	return err
}

// Drain puts the handler into drain mode.
func (s *Server) Drain() {
	atomic.StoreUint32(&s.mode, lnet.StateDrain)
	s.GRPC.GracefulStop()
}

// isDraining checks whether the handler is draining
func (s *Server) isDraining() bool {
	return atomic.LoadUint32(&s.mode) == lnet.StateDrain
}

func (s *Server) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	rinfo := &Info{
		FullMethod: info.FullMethod,
		StartTime:  time.Now(),
	}
	// TODO: Join request context with app context

	var cancel func()
	if s.config.Request.Timeout() > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.config.Request.Timeout())
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Attach app context services to request context
	ctx = config.TreeWithContext(ctx, config.TreeFromContext(s.ctx))
	ctx = log.WithContext(ctx, log.FromContext(s.ctx))
	ctx = stats.WithContext(ctx, stats.FromContext(s.ctx))
	ctx = bg.RegWithContext(ctx, bg.RegFromContext(s.ctx))
	ctx = tracing.WithContext(ctx, tracing.FromContext(s.ctx))
	ctx = disco.AgentWithContext(ctx, disco.AgentFromContext(s.ctx))
	ctx = schedule.SchedulerWithContext(ctx, schedule.SchedulerFromContext(s.ctx))
	ctx = cache.WithContext(ctx, cache.FromContext(s.ctx))

	// Extract Transit and attach transit-specific services
	ctx, err = ExtractTransit(ctx)
	if err != nil {
		return nil, err
	}
	ctx = lcontext.WithTracer(ctx, tracing.FromContext(ctx))
	ctx = lcontext.WithLogger(ctx, log.FromContext(ctx))

	// Extract shipments
	// TODO: Allow to serialise shipments with custom encoder/decoder
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if data, ok := md[shipmentsMD]; ok {
			var shipments []shipment
			err := gob.NewDecoder(bytes.NewReader([]byte(data[0]))).Decode(&shipments)
			if err != nil {
				return nil, err
			}

			for _, s := range shipments {
				ctx = lcontext.WithShipment(ctx, s.Key, s.Value)
			}
		}
	}

	// Build middleware chain and then call it
	next := func(ctx context.Context, info *Info, req interface{}) (interface{}, error) {
		return handler(ctx, req)
	}
	for i := len(s.unaryMiddlewares) - 1; i >= 0; i-- {
		next = s.unaryMiddlewares[i](next)
	}
	return next(ctx, rinfo, req)
}

func (s *Server) streamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	rinfo := &Info{
		FullMethod: info.FullMethod,
		StartTime:  time.Now(),
	}
	// TODO: Join request context with app context

	ctx := ss.Context()

	// Attach app context services to request context
	ctx = config.TreeWithContext(ctx, config.TreeFromContext(s.ctx))
	ctx = log.WithContext(ctx, log.FromContext(s.ctx))
	ctx = stats.WithContext(ctx, stats.FromContext(s.ctx))
	ctx = bg.RegWithContext(ctx, bg.RegFromContext(s.ctx))
	ctx = tracing.WithContext(ctx, tracing.FromContext(s.ctx))
	ctx = disco.AgentWithContext(ctx, disco.AgentFromContext(s.ctx))
	ctx = schedule.SchedulerWithContext(ctx, schedule.SchedulerFromContext(s.ctx))
	ctx = cache.WithContext(ctx, cache.FromContext(s.ctx))

	// Extract Transit and attach transit-specific services
	ctx, err := ExtractTransit(ctx)
	if err != nil {
		return err
	}
	ctx = lcontext.WithTracer(ctx, tracing.FromContext(ctx))
	ctx = lcontext.WithLogger(ctx, log.FromContext(ctx))

	// Extract shipments
	// TODO: Allow to serialise shipments with custom encoder/decoder
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if data, ok := md[shipmentsMD]; ok {
			var shipments []shipment
			err := gob.NewDecoder(bytes.NewReader([]byte(data[0]))).Decode(&shipments)
			if err != nil {
				return err
			}

			for _, s := range shipments {
				ctx = lcontext.WithShipment(ctx, s.Key, s.Value)
			}
		}
	}

	// Wrap context
	ss = &serverStream{
		S: ss,
		C: ctx,
	}

	// Build middleware chain and then call it
	next := func(srv interface{}, _ *Info, ss grpc.ServerStream) error {
		return handler(srv, ss)
	}
	for i := len(s.streamMiddlewares) - 1; i >= 0; i-- {
		next = s.streamMiddlewares[i](next)
	}
	return next(srv, rinfo, ss)
}

// Info contains information about a request
type Info struct {
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
	// StartTime is the time on which the request hast started
	StartTime time.Time
}

type UnaryHandler func(ctx context.Context, info *Info, req interface{}) (interface{}, error)
type UnaryServerMiddleware func(next UnaryHandler) UnaryHandler

type StreamHandler func(srv interface{}, info *Info, ss grpc.ServerStream) error
type StreamServerMiddleware func(next StreamHandler) StreamHandler

type registration func(s *grpc.Server)

type service struct {
	sd *grpc.ServiceDesc
	ss interface{}
}

// serverStream wraps a `grpc.ServerStream` to override its context
type serverStream struct {
	S grpc.ServerStream
	C context.Context
}

func (s *serverStream) SetHeader(md metadata.MD) error {
	return s.S.SetHeader(md)
}
func (s *serverStream) SendHeader(md metadata.MD) error {
	return s.S.SendHeader(md)
}
func (s *serverStream) SetTrailer(md metadata.MD) {
	s.S.SetTrailer(md)
}
func (s *serverStream) Context() context.Context {
	return s.C
}
func (s *serverStream) SendMsg(m interface{}) error {
	return s.S.SendMsg(m)
}
func (s *serverStream) RecvMsg(m interface{}) error {
	return s.S.RecvMsg(m)
}

// mwUnaryServerTracing traces requests with the context `Tracer`
func mwUnaryServerTracing(next UnaryHandler) UnaryHandler {
	return func(ctx context.Context, info *Info, req interface{}) (interface{}, error) {
		tr := lcontext.TransitFromContext(ctx)

		// Extract SpanContext from inbound context
		var spanContext opentracing.SpanContext
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			format := opentracing.HTTPHeaders
			carrier := metadataReaderWriter{md}
			var err error
			spanContext, err = tracing.Extract(ctx, format, carrier)
			switch err {
			case nil:
			case opentracing.ErrSpanContextNotFound:
			default:
				return nil, err
			}
		}

		// Start tracing
		var span opentracing.Span
		span, ctx = tracing.StartSpanFromContext(
			ctx,
			info.FullMethod,
			ext.RPCServerOption(spanContext),
		)
		defer span.Finish()
		span.LogFields(
			olog.String("type", "grpc"),
			olog.String("uuid", tr.UUID()),
			olog.String("id", tr.ShortID()),
			olog.String("startTime", info.StartTime.Format(time.RFC3339Nano)),
		)

		// Next middleware
		res, err := next(ctx, info, req)
		if err != nil {
			span.LogFields(
				olog.Error(err),
			)
		}

		return res, err
	}
}

// mwUnaryServerLogging logs information about HTTP requests/responses
func mwUnaryServerLogging(next UnaryHandler) UnaryHandler {
	return func(ctx context.Context, info *Info, req interface{}) (interface{}, error) {
		logger := log.FromContext(ctx)
		logger.Trace("h.grpc.req.start", "Unary request start",
			log.Type("req", req),
			log.String("full_method", info.FullMethod),
		)

		// Next middleware
		res, err := next(ctx, info, req)

		fields := []log.Field{
			log.Stringer("code", status.Code(err)),
			log.Duration("duration", time.Now().Sub(info.StartTime)),
		}
		if err != nil {
			fields = append(fields, log.Error(err))
		}
		logger.Trace("h.grpc.req.end", "Unary request end", fields...)
		return res, err
	}
}

// mwUnaryServerStats sends the request/response stats
func mwUnaryServerStats(next UnaryHandler) UnaryHandler {
	return func(ctx context.Context, info *Info, req interface{}) (interface{}, error) {
		stats := stats.FromContext(ctx)
		tags := map[string]string{
			"req":         fmt.Sprintf("%T", req),
			"full_method": info.FullMethod,
		}
		stats.Inc("grpc.conc", tags)

		// Next middleware
		res, err := next(ctx, info, req)

		duration := time.Now().Sub(info.StartTime)
		stats.Histogram("grpc.call", 1, tags)
		stats.Timing("grpc.time", duration, tags)
		stats.Dec("grpc.conc", tags)
		return res, err
	}
}

// mwStreamServerTracing traces requests with the context `Tracer`
func mwStreamServerTracing(next StreamHandler) StreamHandler {
	return func(srv interface{}, info *Info, ss grpc.ServerStream) error {
		ctx := ss.Context()
		tr := lcontext.TransitFromContext(ctx)

		// Extract SpanContext from inbound context
		var spanContext opentracing.SpanContext
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			format := opentracing.HTTPHeaders
			carrier := metadataReaderWriter{md}
			var err error
			spanContext, err = tracing.Extract(ctx, format, carrier)
			switch err {
			case nil:
			case opentracing.ErrSpanContextNotFound:
			default:
				return err
			}
		}

		// Start tracing
		var span opentracing.Span
		span, ctx = tracing.StartSpanFromContext(
			ctx,
			info.FullMethod,
			ext.RPCServerOption(spanContext),
		)
		defer span.Finish()
		span.LogFields(
			olog.String("type", "grpc"),
			olog.String("uuid", tr.UUID()),
			olog.String("id", tr.ShortID()),
			olog.String("startTime", info.StartTime.Format(time.RFC3339Nano)),
		)

		// Wrap context
		ss = &serverStream{
			S: ss,
			C: ctx,
		}

		// Next middleware
		if err := next(srv, info, ss); err != nil {
			span.LogFields(
				olog.Error(err),
			)
			return err
		}
		return nil
	}
}

// mwStreamServerLogging logs information about HTTP requests/responses
func mwStreamServerLogging(next StreamHandler) StreamHandler {
	return func(srv interface{}, info *Info, ss grpc.ServerStream) error {
		logger := log.FromContext(ss.Context())
		logger.Trace("h.grpc.req.start", "Stream request start",
			log.String("full_method", info.FullMethod),
		)

		// Next middleware
		err := next(srv, info, ss)

		fields := []log.Field{
			log.Stringer("code", status.Code(err)),
			log.Duration("duration", time.Now().Sub(info.StartTime)),
		}
		if err != nil {
			fields = append(fields, log.Error(err))
		}
		logger.Trace("h.grpc.req.end", "Stream request end", fields...)
		return err
	}
}

// mwStreamServerStats sends the request/response stats
func mwStreamServerStats(next StreamHandler) StreamHandler {
	return func(srv interface{}, info *Info, ss grpc.ServerStream) error {
		stats := stats.FromContext(ss.Context())
		tags := map[string]string{
			"full_method": info.FullMethod,
		}
		stats.Inc("grpc.conc", tags)

		// Next middleware
		err := next(srv, info, ss)

		duration := time.Now().Sub(info.StartTime)
		stats.Histogram("grpc.call", 1, tags)
		stats.Timing("grpc.time", duration, tags)
		stats.Dec("grpc.conc", tags)
		return err
	}
}
