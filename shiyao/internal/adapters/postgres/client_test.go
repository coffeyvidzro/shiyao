package postgres

import "testing"

func TestNewRequiresDatabaseURL(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New succeeded without a database URL")
	}
}

func TestNewParsesDatabaseURLWithoutConnecting(t *testing.T) {
	client, err := New("postgres://shiyao:password@localhost:5432/shiyao?sslmode=disable")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	if client.Conn() != nil {
		t.Fatal("New opened a database connection")
	}
}
