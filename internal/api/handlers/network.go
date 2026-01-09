package handlers

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/amiyamandal-dev/newsp2p/internal/p2p"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// NetworkHandler handles network-related requests
type NetworkHandler struct {
	node   *p2p.P2PNode
	logger *logger.Logger
}

// NewNetworkHandler creates a new network handler
func NewNetworkHandler(node *p2p.P2PNode, logger *logger.Logger) *NetworkHandler {
	return &NetworkHandler{
		node:   node,
		logger: logger.WithComponent("network-handler"),
	}
}

// GetStats returns network statistics
func (h *NetworkHandler) GetStats(c *gin.Context) {
	if h.node == nil {
		response.Success(c, gin.H{
			"status":     "disabled",
			"peer_id":    "",
			"peer_count": 0,
			"addresses":  []string{},
		})
		return
	}

	peerID := h.node.GetPeerID().String()
	peerCount := h.node.GetPeerCount()

	// Get listen addresses with peer ID for sharing
	host := h.node.GetHost()
	addrs := host.Addrs()
	fullAddrs := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), peerID)
		fullAddrs = append(fullAddrs, fullAddr)
	}

	response.Success(c, gin.H{
		"peer_id":    peerID,
		"peer_count": peerCount,
		"status":     "active",
		"addresses":  fullAddrs,
	})
}

// GetPeers returns list of connected peers
func (h *NetworkHandler) GetPeers(c *gin.Context) {
	if h.node == nil {
		response.InternalServerError(c, "P2P node not initialized")
		return
	}

	connectedPeers := h.node.GetConnectedPeers()
	peers := make([]string, len(connectedPeers))
	for i, p := range connectedPeers {
		peers[i] = p.String()
	}

	response.Success(c, gin.H{
		"peers": peers,
		"count": len(peers),
	})
}

// GetPeerInfo returns information about a specific peer
func (h *NetworkHandler) GetPeerInfo(c *gin.Context) {
	if h.node == nil {
		response.InternalServerError(c, "P2P node not initialized")
		return
	}

	idStr := c.Param("id")
	pid, err := peer.Decode(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid peer ID")
		return
	}

	// Check if connected
	connectedness := h.node.GetHost().Network().Connectedness(pid)
	
	// Get addresses
	addrs := h.node.GetHost().Peerstore().Addrs(pid)
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	response.Success(c, gin.H{
		"id":            idStr,
		"connectedness": connectedness.String(),
		"addresses":     addrStrings,
	})
}

// ConnectPeerRequest represents a request to connect to a peer
type ConnectPeerRequest struct {
	Address string `json:"address" binding:"required"`
}

// ConnectPeer connects to a peer by multiaddr
func (h *NetworkHandler) ConnectPeer(c *gin.Context) {
	if h.node == nil {
		response.InternalServerError(c, "P2P node not initialized")
		return
	}

	var req ConnectPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: address is required")
		return
	}

	// Parse multiaddr
	addr, err := multiaddr.NewMultiaddr(req.Address)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid multiaddr: %v", err))
		return
	}

	// Extract peer info
	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid peer address: %v", err))
		return
	}

	// Connect to peer
	if err := h.node.GetHost().Connect(c.Request.Context(), *peerInfo); err != nil {
		h.logger.Error("Failed to connect to peer", "peer", peerInfo.ID, "error", err)
		response.InternalServerError(c, fmt.Sprintf("Failed to connect: %v", err))
		return
	}

	h.logger.Info("Connected to peer manually", "peer", peerInfo.ID)

	response.Success(c, gin.H{
		"message": "Connected successfully",
		"peer_id": peerInfo.ID.String(),
	})
}
