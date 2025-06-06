package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// Ping request tracking
type PingRequest struct {
	Target    *net.IPAddr
	ID        int
	Seq       int
	Size      int
	Timeout   time.Duration
	StartTime time.Time
	ProbeID   int
	ResultCh  chan PingResult
}

type PingResult struct {
	RTT   time.Duration
	Error error
}

// Global ping manager with multiple connections
type PingManager struct {
	connections  map[string]*icmp.PacketConn // key: source IP
	pendingPings map[string]*PingRequest     // key: "target_ip:id:seq"
	mutex        sync.RWMutex
	stats        []*ProbeStats
}

func NewPingManager(stats []*ProbeStats, probes []Probe) (*PingManager, error) {
	pm := &PingManager{
		connections:  make(map[string]*icmp.PacketConn),
		pendingPings: make(map[string]*PingRequest),
		stats:        stats,
	}

	failedConnCount := 0

	// Pre create connections for each unique source IP
	for _, probe := range probes {
		_, err := pm.GetConnection(probe.Src)

		if err != nil {
			failedConnCount++
			log.Printf("%v", err)
		}
	}

	if failedConnCount > 0 && failedConnCount == len(probes) {
		return nil, fmt.Errorf("failed to create ping manager")
	}

	return pm, nil
}

func (pm *PingManager) Close() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for _, conn := range pm.connections {
		conn.Close()
	}
	pm.connections = make(map[string]*icmp.PacketConn)
}

func (pm *PingManager) GetConnection(src string) (*icmp.PacketConn, error) {
	var conn *icmp.PacketConn
	var err error

	srcKey := getSourceKey(src)

	pm.mutex.RLock()
	conn, exists := pm.connections[srcKey]
	pm.mutex.RUnlock()

	if exists {
		return conn, nil
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if shouldUseSourceIP(src) {
		conn, err = icmp.ListenPacket("ip4:icmp", src)
		if err != nil {
			return nil, fmt.Errorf("failed to create ICMP socket with source %s: %v", src, err)
		}
		log.Printf("Created ICMP socket bound to source IP: %s", src)
	} else {
		conn, err = icmp.ListenPacket("ip4:icmp", "")
		if err != nil {
			return nil, fmt.Errorf("failed to create ICMP socket: %v", err)
		}
		log.Printf("Created ICMP socket with auto source IP")
	}

	pm.connections[srcKey] = conn

	// Start response handling goroutine for this connection
	go pm.handleResponses(conn, srcKey)

	return conn, nil
}

func (pm *PingManager) SendPing(probe Probe, id, seq int, timeout time.Duration, probeID int) (time.Duration, error) {
	conn, err := pm.GetConnection(probe.Src)

	if err != nil {
		log.Printf("%v", err)
		return 0, fmt.Errorf("no connection available for source %s", probe.Src)
	}

	// Resolve destination address
	dst, err := net.ResolveIPAddr("ip4", probe.Dst)
	if err != nil {
		return 0, err
	}

	// Create ICMP message
	message := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: make([]byte, probe.Size),
		},
	}

	data, err := message.Marshal(nil)
	if err != nil {
		return 0, err
	}

	// Create ping request record
	resultCh := make(chan PingResult, 1)
	key := fmt.Sprintf("%s:%d:%d", dst.String(), id, seq)

	req := &PingRequest{
		Target:    dst,
		ID:        id,
		Seq:       seq,
		Size:      probe.Size,
		Timeout:   timeout,
		StartTime: time.Now(),
		ProbeID:   probeID,
		ResultCh:  resultCh,
	}

	// Register pending ping
	pm.mutex.Lock()
	pm.pendingPings[key] = req
	pm.mutex.Unlock()

	// Send ping
	_, err = conn.WriteTo(data, dst)
	if err != nil {
		// Clean up registration
		pm.mutex.Lock()
		delete(pm.pendingPings, key)
		pm.mutex.Unlock()
		return 0, err
	}

	// Wait for result or timeout
	select {
	case result := <-resultCh:
		return result.RTT, result.Error
	case <-time.After(timeout):
		// Clean up registration
		pm.mutex.Lock()
		delete(pm.pendingPings, key)
		pm.mutex.Unlock()
		return 0, fmt.Errorf("timeout")
	}
}

func (pm *PingManager) handleResponses(conn *icmp.PacketConn, srcKey string) {
	for {
		reply := make([]byte, 1500)
		_, peer, err := conn.ReadFrom(reply)
		if err != nil {
			continue
		}

		// Parse ICMP response
		replyMsg, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply)
		if err != nil {
			continue
		}

		// Check if it's an Echo Reply
		replyBody, ok := replyMsg.Body.(*icmp.Echo)
		if !ok {
			continue
		}

		// Find corresponding ping request
		key := fmt.Sprintf("%s:%d:%d", peer.String(), replyBody.ID, replyBody.Seq)

		pm.mutex.Lock()
		req, exists := pm.pendingPings[key]
		if exists {
			delete(pm.pendingPings, key)
		}
		pm.mutex.Unlock()

		if exists {
			// Calculate RTT and send result
			rtt := time.Since(req.StartTime)
			select {
			case req.ResultCh <- PingResult{RTT: rtt, Error: nil}:
			default:
				// Channel is full or closed, ignore
			}
		}
	}
}
