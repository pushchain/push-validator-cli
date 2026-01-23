package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
)

func TestRunPeersCore_Success(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	cli := &mockNodeClient{
		peers: []node.Peer{
			{ID: "peer1abc", Addr: "1.2.3.4:26656"},
			{ID: "peer2def", Addr: "5.6.7.8:26656"},
			{ID: "peer3ghi", Addr: "9.10.11.12:26656"},
		},
	}

	err := runPeersCore(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPeersCore_Empty(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	cli := &mockNodeClient{peers: []node.Peer{}}

	err := runPeersCore(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPeersCore_Error(t *testing.T) {
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagNoColor = true
	flagNoEmoji = true

	cli := &mockNodeClient{peersErr: fmt.Errorf("connection refused")}

	err := runPeersCore(context.Background(), cli)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "connection refused") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRPCBase_GenesisDomain(t *testing.T) {
	cfg := config.Config{GenesisDomain: "rpc.push.org"}
	result := resolveRPCBase(cfg)
	if result != "https://rpc.push.org" {
		t.Errorf("resolveRPCBase() = %q, want %q", result, "https://rpc.push.org")
	}
}

func TestResolveRPCBase_GenesisDomainTrailingSlash(t *testing.T) {
	cfg := config.Config{GenesisDomain: "rpc.push.org/"}
	result := resolveRPCBase(cfg)
	if result != "https://rpc.push.org" {
		t.Errorf("resolveRPCBase() = %q, want %q", result, "https://rpc.push.org")
	}
}

func TestResolveRPCBase_RPCLocal(t *testing.T) {
	cfg := config.Config{RPCLocal: "http://localhost:26657"}
	result := resolveRPCBase(cfg)
	if result != "http://localhost:26657" {
		t.Errorf("resolveRPCBase() = %q, want %q", result, "http://localhost:26657")
	}
}

func TestResolveRPCBase_Default(t *testing.T) {
	cfg := config.Config{}
	result := resolveRPCBase(cfg)
	if result != "http://127.0.0.1:26657" {
		t.Errorf("resolveRPCBase() = %q, want %q", result, "http://127.0.0.1:26657")
	}
}
