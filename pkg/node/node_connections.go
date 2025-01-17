package node

import (
	"sync"

	"google.golang.org/grpc/connectivity"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
)

// Connections stores a map
// of a node to grpc connection
type Connections struct {
	// Revisit this idea at some point,
	// on paper this seems like a good fit
	// but maybe there's something I am missing
	nodeToConn sync.Map
}

// NewNodeConnectionManager creates a new
// NodeConnection instance
func NewNodeConnectionManager() *Connections {
	return &Connections{}
}

// CreateConnection creates a connection to a grpc endpoint
// and stores it in the nodeToConn map with a mapping of
// nodeID -> conn
func (n *Connections) CreateConnection(nodeID uuid.UUID, address string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	for {
		if conn.GetState() == connectivity.Ready {
			n.nodeToConn.Store(nodeID, conn)
			break
		}
	}

	return conn, nil
}

// GetConnection receives a nodeID and returns its connection.
// If a connection is not available return nil
func (n *Connections) GetConnection(nodeID uuid.UUID) *grpc.ClientConn {
	v, ok := n.nodeToConn.Load(nodeID)
	if !ok {
		return nil
	}

	return v.(*grpc.ClientConn)
}

// Close receives a nodeID and closes its connection
// and removes it from the map.
// If a connection is not available return nil
func (n *Connections) Close(nodeID uuid.UUID) error {
	v, ok := n.nodeToConn.Load(nodeID)
	if !ok {
		return nil
	}

	conn := v.(*grpc.ClientConn)
	err := conn.Close()

	if err != nil {
		return err
	}

	n.nodeToConn.Delete(nodeID)

	return nil
}

// Shutdown closes all the connections
func (n *Connections) Shutdown() []error {
	errors := make([]error, 0, 0)
	n.nodeToConn.Range(func(key, value interface{}) bool {
		err := value.(*grpc.ClientConn).Close()

		if err != nil {
			errors = append(errors, err)
		}

		return true
	})

	return errors
}
