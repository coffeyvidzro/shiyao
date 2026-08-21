package redis

import (
	"bufio"
	"context"
	"net"
	"testing"
)

func TestNewParsesURL(t *testing.T) {
	client, err := New("rediss://user:secret@example.test:6380/2")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if client.address != "example.test:6380" || client.username != "user" || client.password != "secret" || client.database != 2 || !client.useTLS {
		t.Fatalf("unexpected client: %+v", client)
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	for _, rawURL := range []string{"", "http://localhost", "redis://localhost/not-a-number", "redis:///0"} {
		if _, err := New(rawURL); err == nil {
			t.Errorf("New(%q) succeeded", rawURL)
		}
	}
}

func TestPing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		if _, err := reader.ReadString('\n'); err != nil {
			serverDone <- err
			return
		}
		for range 1 {
			if _, err := reader.ReadString('\n'); err != nil {
				serverDone <- err
				return
			}
			if _, err := reader.ReadString('\n'); err != nil {
				serverDone <- err
				return
			}
		}
		_, err = conn.Write([]byte("+PONG\r\n"))
		serverDone <- err
	}()

	client, err := New("redis://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("redis test server: %v", err)
	}
}
