package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	bhost "github.com/libp2p/go-libp2p-blankhost"
	csms "github.com/libp2p/go-conn-security-multistream"
	pstoremem "github.com/libp2p/go-libp2p-peerstore/pstoremem"
	rendezvous "github.com/libp2p/go-libp2p-rendezvous"
	db "github.com/libp2p/go-libp2p-rendezvous/db/sqlite"
	secio "github.com/libp2p/go-libp2p-secio"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	yamux "github.com/libp2p/go-libp2p-yamux"
	msmux "github.com/libp2p/go-stream-muxer-multistream"
	tcp "github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
)

const PingProto = protocol.ID("/ping/1.0.0")

// ─── Minimal Redis client (no external dependency) ───────────────────────────

type RedisClient struct{ addr string }

func (c *RedisClient) Set(key, val string) error {
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	cmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(key), key, len(val), val)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("unexpected redis SET response: %s", line)
	}
	return nil
}

func (c *RedisClient) Get(key string) (string, error) {
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cmd := fmt.Sprintf("*2\r\n$3\r\nGET\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", err
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(line, "$-1") {
		return "", nil // key not found
	}
	if strings.HasPrefix(line, "$") {
		var length int
		if _, err := fmt.Sscanf(line, "$%d", &length); err != nil {
			return "", err
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	}
	return "", fmt.Errorf("unexpected redis GET response: %s", line)
}

// ─── libp2p host helpers ──────────────────────────────────────────────────────

func getContainerIP(redisAddr string) string {
	conn, err := net.Dial("udp", redisAddr)
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func makeHost(ctx context.Context, ip string, port int) (host.Host, error) {
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, err
	}
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	ps := pstoremem.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)

	sw := swarm.NewSwarm(ctx, pid, ps, nil)

	secMuxer := new(csms.SSMuxer)
	secMuxer.AddTransport(secio.ID, &secio.Transport{
		LocalID:    pid,
		PrivateKey: priv,
	})

	stMuxer := msmux.NewBlankTransport()
	stMuxer.AddTransport("/yamux/1.0.0", yamux.DefaultTransport)

	upgrader := &tptu.Upgrader{
		Secure:  secMuxer,
		Muxer:   stMuxer,
		Filters: sw.Filters,
	}

	tcpT := tcp.NewTCPTransport(upgrader)
	if err := sw.AddTransport(tcpT); err != nil {
		return nil, err
	}

	listenAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ip, port))
	if err := sw.Listen(listenAddr); err != nil {
		return nil, err
	}

	h := bhost.NewBlankHost(sw)

	// ping responder (used by discoverer to verify reachability)
	h.SetStreamHandler(PingProto, func(s network.Stream) {
		buf := make([]byte, 4)
		if _, err := io.ReadFull(s, buf); err != nil {
			s.Reset()
			return
		}
		if string(buf) == "ping" {
			_, _ = s.Write([]byte("pong"))
		}
		s.Close()
	})

	return h, nil
}

func peerMultiaddr(h host.Host) string {
	for _, a := range h.Addrs() {
		return fmt.Sprintf("%s/p2p/%s", a.String(), h.ID().Pretty())
	}
	return ""
}

// ─── Roles ───────────────────────────────────────────────────────────────────

func runServer(ctx context.Context, h host.Host, r *RedisClient, testKey string) {
	dbi, err := db.OpenDB(ctx, ":memory:")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open sqlite db: %v\n", err)
		os.Exit(1)
	}
	defer dbi.Close()

	_ = rendezvous.NewRendezvousService(h, dbi)

	serverKey := fmt.Sprintf("%s_server_addr", testKey)
	if err := r.Set(serverKey, peerMultiaddr(h)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write server addr to redis: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Rendezvous server ready at %s\n", peerMultiaddr(h))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func runRegistrant(ctx context.Context, h host.Host, r *RedisClient, testKey string) {
	serverKey := fmt.Sprintf("%s_server_addr", testKey)
	serverAddr := waitForKey(r, serverKey, 60*time.Second)
	if serverAddr == "" {
		fmt.Fprintf(os.Stderr, "Timeout waiting for server address\n")
		os.Exit(1)
	}

	serverInfo := mustAddrInfo(serverAddr)
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	if err := h.Connect(connectCtx, *serverInfo); err != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "Failed to connect to rendezvous server: %v\n", err)
		os.Exit(1)
	}
	cancel()

	rp := rendezvous.NewRendezvousPoint(h, serverInfo.ID)
	ns := fmt.Sprintf("rendezvous-ns-%s", testKey)
	grantedTTL, err := rp.Register(ctx, ns, 300)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Registered in namespace %q with TTL %v\n", ns, grantedTTL)

	_ = r.Set(fmt.Sprintf("%s_registrant_id", testKey), h.ID().Pretty())
	_ = r.Set(fmt.Sprintf("%s_registrant_done", testKey), "1")

	fmt.Println("Registrant waiting for discoverer...")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func runDiscoverer(ctx context.Context, h host.Host, r *RedisClient, testKey string) {
	serverKey := fmt.Sprintf("%s_server_addr", testKey)
	regDoneKey := fmt.Sprintf("%s_registrant_done", testKey)

	// wait for both server and registrant to be ready
	deadline := time.Now().Add(60 * time.Second)
	var serverAddr, regDone string
	for time.Now().Before(deadline) {
		if serverAddr == "" {
			serverAddr, _ = r.Get(serverKey)
		}
		if regDone == "" {
			regDone, _ = r.Get(regDoneKey)
		}
		if serverAddr != "" && regDone != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if serverAddr == "" || regDone == "" {
		fmt.Println("error: Timeout waiting for server and registrant")
		fmt.Println("status: fail")
		os.Exit(1)
	}

	expectedID, _ := r.Get(fmt.Sprintf("%s_registrant_id", testKey))

	serverInfo := mustAddrInfo(serverAddr)
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	if err := h.Connect(connectCtx, *serverInfo); err != nil {
		cancel()
		fmt.Printf("error: Failed to connect to server: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}
	cancel()

	rp := rendezvous.NewRendezvousPoint(h, serverInfo.ID)
	ns := fmt.Sprintf("rendezvous-ns-%s", testKey)
	regs, _, err := rp.Discover(ctx, ns, 100, nil)
	if err != nil {
		fmt.Printf("error: Discover failed: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}
	fmt.Printf("Discovered %d peer(s)\n", len(regs))

	var target *rendezvous.Registration
	for i := range regs {
		if regs[i].Peer.ID.Pretty() == expectedID {
			target = &regs[i]
			break
		}
	}
	if target == nil {
		fmt.Printf("error: Expected peer %s not found in discovery results\n", expectedID)
		fmt.Println("status: fail")
		os.Exit(1)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

	if err := h.Connect(dialCtx, target.Peer); err != nil {
		fmt.Printf("error: Dial to registrant failed: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}

	s, err := h.NewStream(dialCtx, target.Peer.ID, PingProto)
	if err != nil {
		fmt.Printf("error: Open stream failed: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}
	defer s.Close()

	if _, err := s.Write([]byte("ping")); err != nil {
		fmt.Printf("error: Write ping failed: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}

	buf := make([]byte, 4)
	if _, err := io.ReadFull(s, buf); err != nil {
		fmt.Printf("error: Read pong failed: %v\n", err)
		fmt.Println("status: fail")
		os.Exit(1)
	}

	if string(buf) == "pong" {
		fmt.Println("status: pass")
		os.Exit(0)
	}
	fmt.Printf("error: Unexpected response: %s\n", string(buf))
	fmt.Println("status: fail")
	os.Exit(1)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func waitForKey(r *RedisClient, key string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		val, _ := r.Get(key)
		if val != "" {
			return val
		}
		time.Sleep(500 * time.Millisecond)
	}
	return ""
}

func mustAddrInfo(addrStr string) *peer.AddrInfo {
	maddr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse multiaddr %q: %v\n", addrStr, err)
		os.Exit(1)
	}
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract AddrInfo from %q: %v\n", addrStr, err)
		os.Exit(1)
	}
	return info
}

// ─── Entry point ─────────────────────────────────────────────────────────────

func main() {
	role := os.Getenv("ROLE")
	redisAddr := os.Getenv("REDIS_ADDR")
	testKey := os.Getenv("TEST_KEY")

	if role == "" || redisAddr == "" || testKey == "" {
		fmt.Fprintln(os.Stderr, "Required env vars: ROLE, REDIS_ADDR, TEST_KEY")
		os.Exit(1)
	}

	r := &RedisClient{addr: redisAddr}
	containerIP := getContainerIP(redisAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, err := makeHost(ctx, containerIP, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create host: %v\n", err)
		os.Exit(1)
	}
	defer h.Close()

	fmt.Printf("Node started | role=%s addr=%s\n", role, peerMultiaddr(h))

	switch role {
	case "server":
		runServer(ctx, h, r, testKey)
	case "registrant":
		runRegistrant(ctx, h, r, testKey)
	case "discoverer":
		runDiscoverer(ctx, h, r, testKey)
	default:
		fmt.Fprintf(os.Stderr, "Unknown role: %s\n", role)
		os.Exit(1)
	}
}
