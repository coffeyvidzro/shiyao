package websocket

import (
	"context"
	"log"

	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type ExecStreamer interface {
	ExecStream(
		ctx context.Context,
		sandboxID string,
		req vsock.ExecRequest,
		receive func(vsock.ExecFrame) error,
	) (vsock.ExecResult, error)
}

func BridgeExecStream(
	ctx context.Context,
	conn *Conn,
	sandboxID string,
	streamer ExecStreamer,
) {
	defer conn.Close()

	var clientMsg ClientMessage
	if err := conn.ReadJSON(&clientMsg); err != nil {
		log.Printf("failed to read initial exec message: %v", err)
		return
	}

	if clientMsg.Type != TypeExec {
		_ = conn.WriteJSON(ServerMessage{
			Type:  TypeError,
			Error: "first message must be of type 'exec'",
		})
		return
	}

	req := vsock.ExecRequest{
		Version:   vsock.ProtocolVersion,
		ID:        sandboxID,
		Command:   clientMsg.Command,
		Args:      clientMsg.Args,
		Env:       clientMsg.Env,
		TimeoutMS: clientMsg.TimeoutMS,
		Stream:    true,
	}

	result, err := streamer.ExecStream(ctx, sandboxID, req, func(frame vsock.ExecFrame) error {
		var msgType string
		switch frame.Stream {
		case TypeStdout:
			msgType = TypeStdout
		case TypeStderr:
			msgType = TypeStderr
		default:
			return nil
		}

		return conn.WriteJSON(ServerMessage{
			Type: msgType,
			Data: frame.Data,
		})
	})
	if err != nil {
		_ = conn.WriteJSON(ServerMessage{Type: TypeError, Error: err.Error()})
		return
	}

	_ = conn.WriteJSON(ServerMessage{
		Type: TypeResult,
		Result: &ExecResult{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			Error:    result.Error,
		},
	})
}
