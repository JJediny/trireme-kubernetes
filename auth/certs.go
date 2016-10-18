package auth

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/aporeto-inc/kubernetes-integration/kubernetes"
	"github.com/aporeto-inc/trireme/interfaces"
	"github.com/golang/glog"
)

// Certs is used to monitor the Certificate used all over the Kubernetes Cluster.
type Certs struct {
	publicKeyAdder    interfaces.PublicKeyAdder
	nodeResultChan    chan watch.Event
	nodeStopChan      chan bool
	certStopChan      chan bool
	nodeAnnotationKey string
}

// NewCertsWatcher creates a new Certs object and start watching for changes and updates
// on all the nodes on the Kube Cluster.
func NewCertsWatcher(client kubernetes.Client, pki interfaces.PublicKeyAdder, nodeAnnotationKey string) *Certs {
	// Creating all the channels.
	certs := &Certs{
		publicKeyAdder:    pki,
		nodeResultChan:    make(chan watch.Event),
		nodeStopChan:      make(chan bool),
		certStopChan:      make(chan bool),
		nodeAnnotationKey: nodeAnnotationKey,
	}

	// This will start to enqueue new Event nodes.
	go client.NodeWatcher(certs.nodeResultChan, certs.nodeStopChan)

	return certs
}

// StartWatchingCerts processes all the events for certs.
func (c *Certs) StartWatchingCerts() {
	for {
		select {
		case <-c.certStopChan:
			glog.V(2).Infof("Received Stop signal for Certs")
			return
		case req := <-c.nodeResultChan:
			glog.V(2).Infof("Processing NodeEvents")
			c.ProcessNodeUpdate(req.Object.(*api.Node), req.Type)
		}
	}
}

// ProcessNodeUpdate is triggered when a new event is received.
func (c *Certs) ProcessNodeUpdate(node *api.Node, eventType watch.EventType) {
	if eventType == watch.Added {
		annotations := node.GetAnnotations()
		if cert, ok := annotations[c.nodeAnnotationKey]; ok {
			c.addCertToCache(node.GetName(), certStringToBytes(cert))
		}
	}
}

// StopWatchingCerts stops watching for new certs and stops all the routines.
func (c *Certs) StopWatchingCerts() {
	c.nodeStopChan <- true
	c.certStopChan <- true
}

// RegisterPKI registers the Cert of this node as an annotation on the KubeAPI.
func (c *Certs) RegisterPKI(client kubernetes.Client, cert []byte) {
	client.AddLocalNodeAnnotation(c.nodeAnnotationKey, certBytesToString(cert))
}

func certBytesToString(cert []byte) string {
	return string(cert)
}

func certStringToBytes(cert string) []byte {
	return []byte(cert)
}

// SyncNodeCerts syncs all the nodes on the Kube Cluster and register the
// certs written as annotations.
func (c *Certs) SyncNodeCerts(client kubernetes.Client) error {
	allNodes, err := client.AllNodes()
	if err != nil {
		return fmt.Errorf("Couldn't Sync certs: %s", err)
	}
	for _, node := range allNodes.Items {
		annotations := node.GetAnnotations()
		if cert, ok := annotations[c.nodeAnnotationKey]; ok {
			c.addCertToCache(node.GetName(), certStringToBytes(cert))
		}
	}
	return nil
}

func (c *Certs) addCertToCache(nodeName string, cert []byte) {
	glog.V(2).Infof("Adding cert for node: %s", nodeName)
	c.publicKeyAdder.PublicKeyAdd(nodeName, cert)
}
