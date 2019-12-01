package grpcerrors_test

import (
	context "context"
	"fmt"
	"testing"
	"time"

	"github.com/deixis/errors"
	"github.com/deixis/errors/grpcerrors"
	lgrpc "github.com/deixis/spine/net/grpc"
	lt "github.com/deixis/spine/testing"
	"google.golang.org/grpc"
)

func TestClientServer(t *testing.T) {
	tt := lt.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build server
	h := lgrpc.NewServer()
	h.RegisterService(&_Test_serviceDesc, &MyTestServer{
		t: tt,
	})
	target := startServer(ctx, h)

	// Build client
	conn, err := grpc.DialContext(ctx, target, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		t.Fatal(ctx.Err())
	}
	testClient := NewTestClient(conn)

	_, err = testClient.Hello(ctx, &Request{Msg: "Ping"})
	if err == nil {
		t.Fatal("expect to receive an error")
	}
	err = grpcerrors.Unpack(err)

	switch err := err.(type) {
	case *errors.BadRequest:
		if len(err.Violations) != 1 {
			t.Fatalf("expect to have 1 violation, but got %d", len(err.Violations))
		}
		violation := err.Violations[0]
		expectField := "foo"
		expectDescription := "Missing data"
		if expectField != violation.Field {
			t.Errorf("expect to have Field violation %s, but got %s",
				expectField, violation.Field,
			)
		}
		if expectDescription != violation.Description {
			t.Errorf("expect to have Description violation %s, but got %s",
				expectDescription, violation.Description,
			)
		}
	default:
		t.Errorf("unexpected error %v", err)
	}

	h.Drain()
}

type MyTestServer struct {
	t *lt.T
}

func (s *MyTestServer) Hello(ctx context.Context, req *Request) (*Response, error) {
	bad := errors.Bad(&errors.FieldViolation{
		Field:       "foo",
		Description: "Missing data",
	})
	return nil, grpcerrors.Pack(bad).Err()
}

// func (s *MyTestServer) HelloFlow(
// 	stream Test_HelloFlowServer,
// ) error {
// 	log.Trace(stream.Context(), "test.hello", "Calling Hello")

// 	expectLang := "en_GB"
// 	lang, ok := lcontext.Shipment(stream.Context(), "lang").(string)
// 	if !ok || lang != expectLang {
// 		s.t.Errorf("expect lang %s from shipment, but got %s", expectLang, lang)
// 	}

// 	for {
// 		req, err := stream.Recv()
// 		switch err {
// 		case nil, io.EOF:
// 		default:
// 			s.t.Fatal(err)
// 		}
// 		if err == io.EOF {
// 			break
// 		}

// 		expectMsg := "Ping"
// 		if expectMsg != req.Msg {
// 			s.t.Errorf("expect to get %s, but got %s", expectMsg, req.Msg)
// 		}
// 	}

// 	if err := stream.Send(&Response{Msg: "Pong"}); err != nil {
// 		s.t.Fatal(err)
// 	}
// 	if err := stream.Send(&Response{Msg: "Pong"}); err != nil {
// 		s.t.Fatal(err)
// 	}
// 	return nil
// }

func startServer(ctx context.Context, h *lgrpc.Server) string {
	addr := fmt.Sprintf("localhost:%d", lt.NextPort())

	// Start serving requests
	go func() {
		err := h.Serve(ctx, addr)
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(50 * time.Millisecond)

	return addr
}
