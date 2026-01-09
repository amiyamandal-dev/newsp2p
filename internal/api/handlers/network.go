package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/peer"

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
		response.InternalServerError(c, "P2P node not initialized")
		return
	}

	peerID := h.node.GetPeerID().String()
	peerCount := h.node.GetPeerCount()

	response.Success(c, gin.H{
		"peer_id":    peerID,
		"peer_count": peerCount,
		"status":     "active",
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
