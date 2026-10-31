package clabernetes

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-logr/logr/funcr"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	defaultNamespace = "default"
	pollInterval     = 2 * time.Second

	envKubeconfig = "CLAB_KUBECONFIG"
	envContext    = "CLAB_KUBE_CONTEXT"
	envNamespace  = "CLAB_KUBE_NAMESPACE"

	c9sAPIVersion         = "c9s.run/v1alpha1"
	labelRuntime          = "containerlab.dev/runtime"
	labelApp              = "c9s.run/app"
	labelTopologyOwner    = "c9s.run/topologyOwner"
	labelTopologyNode     = "c9s.run/topologyNode"
	labelOriginalNodeName = "c9s.run/originalNodeName"
	labelIgnoreReconcile  = "c9s.run/ignoreReconcile"
	clabernetesAppValue   = "clabernetes"
	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"
	// persistenceReclaimRetain keeps a node's persistent claim when its Node is deleted.
	persistenceReclaimRetain = "Retain"
	// defaultContainerAnnotation names the primary device container of a c9s pod.
	defaultContainerAnnotation = "kubectl.kubernetes.io/default-container"
)

var topologyGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "topologies",
}

var nodeGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "nodes",
}

var linkGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "links",
}

var nodeProfileGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "nodeprofiles",
}

type Runtime struct {
	client     dynamic.Interface
	kubeClient kubernetes.Interface
	restConfig *rest.Config
	namespace  string
	// labNamespaceOverride is set by --namespace or CLAB_KUBE_NAMESPACE. When
	// empty, lab-scoped operations derive their namespace from the lab name.
	labNamespaceOverride string
	timeout              time.Duration
	// kindLinkApplyMode resolves the default link apply mode a node kind declares.
	// The plan uses it to predict the pod lifecycle impact of link changes.
	kindLinkApplyMode func(kind string) string
	// clients loads the Kubernetes clients on first use. Commands that only compile the
	// topology (validate, deploy --emit-crs) never call it, so they work without a
	// kubeconfig that loads. It is nil when the clients were injected (tests).
	clients *clientLoader
	// connected marks a runtime view whose client fields are populated.
	connected bool
	// identity is the configured identity of calls without one; in a connected view it is the
	// identity the clients impersonate.
	identity clablabruntime.Identity
}

// kubeClients is the set of Kubernetes clients a runtime view operates with.
type kubeClients struct {
	dynamic    dynamic.Interface
	kube       kubernetes.Interface
	restConfig *rest.Config
}

// clientLoader loads the kubeconfig once and builds, and caches, the Kubernetes clients of
// every identity from it.
type clientLoader struct {
	loadConfig func() (*rest.Config, string, error)
	newClients func(*rest.Config) (*kubeClients, error)

	once      sync.Once
	base      *rest.Config
	namespace string
	err       error

	mu         sync.Mutex
	byIdentity map[string]*kubeClients
}

func newClientLoader() *clientLoader {
	return &clientLoader{
		loadConfig: kubeClientConfig,
		newClients: newKubeClients,
	}
}

func newKubeClients(restConfig *rest.Config) (*kubeClients, error) {
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &kubeClients{dynamic: client, kube: kubeClient, restConfig: restConfig}, nil
}

// load returns the clients of an identity. The impersonation is part of the rest config, so it
// applies to every transport built from it: REST calls, watches, log streams, and the SPDY and
// websocket upgrades of exec, attach and port-forward.
func (l *clientLoader) load(
	identity clablabruntime.Identity,
) (*kubeClients, string, error) {
	l.once.Do(func() {
		l.base, l.namespace, l.err = l.loadConfig()
		if l.namespace == "" {
			l.namespace = defaultNamespace
		}
	})
	if l.err != nil {
		return nil, "", l.err
	}

	key := identityKey(identity)

	l.mu.Lock()
	defer l.mu.Unlock()

	if clients, ok := l.byIdentity[key]; ok {
		return clients, l.namespace, nil
	}

	restConfig := rest.CopyConfig(l.base)
	if !identity.IsZero() {
		restConfig.Impersonate = rest.ImpersonationConfig{
			UserName: identity.User,
			Groups:   slices.Clone(identity.Groups),
		}
	}

	clients, err := l.newClients(restConfig)
	if err != nil {
		return nil, "", err
	}
	if l.byIdentity == nil {
		l.byIdentity = map[string]*kubeClients{}
	}
	l.byIdentity[key] = clients

	return clients, l.namespace, nil
}

func identityKey(identity clablabruntime.Identity) string {
	return identity.User + "\x00" + strings.Join(identity.Groups, "\x00")
}

// connect returns a view of the runtime with the Kubernetes clients of an identity populated,
// loading the kubeconfig on first use. A zero identity uses the runtime's configured identity.
// Every operation that reads or changes cluster state calls it first.
func (r *Runtime) connect(identity clablabruntime.Identity) (*Runtime, error) {
	if identity.IsZero() {
		identity = r.identity
	}
	if r.connected && identity.Equal(r.identity) {
		return r, nil
	}
	if r.clients == nil {
		// clients were injected (tests); they act as the identity they were built with
		return r, nil
	}

	clients, namespace, err := r.clients.load(identity)
	if err != nil {
		return nil, err
	}

	view := *r
	view.client = clients.dynamic
	view.kubeClient = clients.kube
	view.restConfig = clients.restConfig
	if view.namespace == "" {
		view.namespace = namespace
	}
	view.identity = identity
	view.connected = true

	return &view, nil
}

func withKubernetesClientDebugLogs(ctx context.Context) context.Context {
	verbosity := -1
	if log.GetLevel() <= log.DebugLevel {
		verbosity = 3
	}
	logger := funcr.New(func(prefix, args string) {
		log.Debug("Kubernetes client", "log", strings.TrimSpace(prefix+" "+args))
	}, funcr.Options{Verbosity: verbosity})

	return klog.NewContext(ctx, logger)
}

func init() {
	clablabruntime.Register(clablabruntime.ClabernetesRuntimeName, New)
}

// New returns the c9s lab runtime. It does not load the kubeconfig: the Kubernetes clients
// are created on the first operation that needs the cluster.
func New(cfg clablabruntime.Config) (clablabruntime.LabRuntime, error) {
	return &Runtime{
		labNamespaceOverride: configuredLabNamespace(cfg.Namespace),
		timeout:              cfg.Timeout,
		kindLinkApplyMode:    cfg.KindLinkApplyMode,
		identity:             cfg.Identity,
		clients:              newClientLoader(),
	}, nil
}
