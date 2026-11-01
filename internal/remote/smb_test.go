package remote

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// TestSMBIntegration 对真实 SMB 服务端做读写删除往返。
// 需要设置 GOCOMPARE_SMB_TEST_URL，例如 smb://user:pass@127.0.0.1:1445/share/gocompare-test.txt；
// 未设置时跳过，CI 中没有 SMB 服务端。
func TestSMBIntegration(t *testing.T) {
	raw := os.Getenv("GOCOMPARE_SMB_TEST_URL")
	if raw == "" {
		t.Skip("GOCOMPARE_SMB_TEST_URL not set")
	}
	store, path, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err = store.Write(ctx, path, []byte("a much longer original body")); err != nil {
		t.Fatal(err)
	}
	if err = store.Write(ctx, path, []byte("short")); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(ctx, path)
	if err != nil || string(got) != "short" {
		t.Fatalf("overwrite must truncate: %q %v", got, err)
	}
	if err = store.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(ctx, path); err == nil {
		t.Fatal("deleted file still readable")
	}
}

func TestSMBCancelDuringNegotiate(t *testing.T) {
	// 服务端接受连接但从不应答 NEGOTIATE，只能靠取消返回。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	client := &SMB{Addr: ln.Addr().String(), Share: "share", User: "u", Pass: "p"}
	done := make(chan error, 1)
	go func() { _, err := client.Read(ctx, "a.txt"); done <- err }()
	select {
	case err = <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want deadline exceeded, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("negotiate ignored context cancellation")
	}
}
