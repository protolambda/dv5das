package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/protolambda/ask"
	"io"
	"net"
	"net/http"
	"time"
)

type NodeCmd struct {
	Priv       *P2pPrivKeyFlag `ask:"--priv" help:"Private key, in raw hex encoded format"`
	ENRIP      net.IP          `ask:"--enr-ip" help:"IP to put in ENR"`
	ENRUDP     uint16          `ask:"--enr-udp" help:"UDP port to put in ENR"`
	ListenIP   net.IP          `ask:"--listen-ip" help:"Listen IP."`
	ListenUDP  uint16          `ask:"--listen-udp" help:"Listen UDP port. Will try ENR port otherwise."`
	APIAddr    string          `ask:"--api-addr" help:"Address to bind HTTP API server to. API is disabled if empty."`
	NodeDBPath string          `ask:"--node-db" help:"Path to dv5 node DB. Memory DB if empty."`
	Bootnodes  []string        `ask:"--bootnodes" help:"Optionally befriend other bootnodes"`
	LogCmd     `ask:".log" help:"Logging options"`

	log log.Logger
	dv5 *discover.UDPv5
	srv *http.Server

	dasTree TreeNode
}

func (nc *NodeCmd) Help() string {
	return "Run DAS prototype node."
}

func (nc *NodeCmd) Default() {
	nc.ListenIP = net.IPv4zero
	nc.APIAddr = "0.0.0.0:8000"
}

func (nc *NodeCmd) Run(ctx context.Context, args ...string) error {
	nc.log = nc.LogCmd.Create()

	bootNodes := make([]*enode.Node, 0, len(nc.Bootnodes))
	for i := 0; i < len(nc.Bootnodes); i++ {
		dv5Addr, err := ParseEnrOrEnode(nc.Bootnodes[i])
		if err != nil {
			return fmt.Errorf("bootnode %d is bad: %v", i, err)
		}
		bootNodes = append(bootNodes, dv5Addr)
	}

	if nc.Priv == nil {
		return fmt.Errorf("need p2p priv key")
	}

	ecdsaPrivKey := (*ecdsa.PrivateKey)(nc.Priv)

	if nc.ListenUDP == 0 {
		nc.ListenUDP = nc.ENRUDP
	}

	udpAddr := &net.UDPAddr{
		IP:   nc.ListenIP,
		Port: int(nc.ListenUDP),
	}

	localNodeDB, err := enode.OpenDB(nc.NodeDBPath)
	if err != nil {
		return err
	}
	localNode := enode.NewLocalNode(localNodeDB, ecdsaPrivKey)
	if nc.ENRIP != nil {
		localNode.SetStaticIP(nc.ENRIP)
	}
	if nc.ENRUDP != 0 {
		localNode.SetFallbackUDP(int(nc.ENRUDP))
	}

	nc.log.Info("created local discv5 identity", "enr", localNode.Node().String())

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	router := http.NewServeMux()
	nc.srv = &http.Server{
		Addr:    nc.APIAddr,
		Handler: router,
	}
	router.HandleFunc("/enr", func(w http.ResponseWriter, req *http.Request) {
		nc.log.Info("received ENR API request", "remote", req.RemoteAddr)
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(200)
		enr := localNode.Node().String()
		if _, err := io.WriteString(w, enr); err != nil {
			nc.log.Error("failed to respond to request from", "remote", req.RemoteAddr, "err", err)
		}
	})

	go func() {
		nc.log.Info("starting API server, ENR reachable on: http://" + nc.srv.Addr + "/enr")
		if err := nc.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			nc.log.Error("API server listen failure", "err", err)
		}
	}()

	cfg := discover.Config{
		PrivateKey:   ecdsaPrivKey,
		NetRestrict:  nil,
		Bootnodes:    bootNodes,
		Unhandled:    nil, // Not used in dv5
		Log:          nc.log,
		ValidSchemes: enode.ValidSchemes,
	}
	udpV5, err := discover.ListenV5(conn, localNode, cfg)
	if err != nil {
		return err
	}
	nc.dv5 = udpV5

	// init tree with our own identity (TODO: persist tree on disk for later retrieval on restart)
	nc.dasTree = &LeafNode{depth: 0, score: 0, self: localNode.Node()}

	// TODO: start tree balancing work

	// TODO: start sample request work

	// TODO: data-blobs source, publish canonical KZG commitments somewhere (gossipsub?)

	// TODO: seed samples

	udpV5.RegisterTalkHandler("das", nc.OnDasRequest)

	return nil
}

//  OnDasRequest implements discover.RegisterTalkHandler
func (nc *NodeCmd) OnDasRequest(id enode.ID, addr *net.UDPAddr, bytes []byte) []byte {
	// TODO handle request types:
	//  - bundle to split and relay in sub-bundles
	//  - sample request
	return nil
}

func (nc *NodeCmd) Close() error {
	nc.dv5.Close()
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	return nc.srv.Shutdown(ctx)
}

func main() {
	ask.Run(new(NodeCmd))
}
