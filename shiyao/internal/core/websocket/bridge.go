package websocket

import (
	"context"
	"log"

	"github.com/coffeyvidzro/shiyao/internal/core/vsock"
)

// ExecStreamer is an interface to abstract the VSOCK execution logic.
// This allows you to mock it in tests and keeps the websocket package decoupled.
type ExecStreamer interface {
	ExecStream(
		ctx context.Context,
		sandboxID string,
		req vsock.ExecRequest,
		receive func(vsock.ExecFrame) error,
	) (vsock.ExecResult, error)
}

// BridgeExecStream handles the full lifecycle of a WebSocket execution session.
func BridgeExecStream(
	ctx context.Context,
	conn *Conn,
	sandboxID string,
	streamer ExecStreamer,
) {
	defer conn.Close()

	// 1. Read the initial exec request from the client
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

	// 2. Dynamically set the deadline based on the requested execution timeout
	conn.SetExecutionDeadline(clientMsg.TimeoutMS)

	// 3. Convert to VSOCK ExecRequest
	req := vsock.ExecRequest{
		Version:   vsock.ProtocolVersion,
		ID:        sandboxID,
		Command:   clientMsg.Command,
		Args:      clientMsg.Args,
		Env:       clientMsg.Env,
		TimeoutMS: clientMsg.TimeoutMS,
		Stream:    true,
	}

	// 4. Stream the execution output back to the client
	// CRITICAL: Return the error from WriteJSON to break the loop if the client disconnects.
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

	// 5. Handle errors or send the final result
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
