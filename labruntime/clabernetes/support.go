package clabernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// requiredClabernetes names the clabernetes release the runtime needs. Node run state
// (nodes.c9s.run spec.runState with restart requests) and the clabwire capture contract
// (--nodeName, repeated --interface, --format pcapng, --list-interfaces) ship together, so one
// check of the run state field covers both.
const requiredClabernetes = "clabernetes with Node run state and the clabwire capture contract " +
	"(0.11 or newer)"

// c9sOpenAPIPath is the OpenAPI v3 document of the c9s.run/v1alpha1 group. The API server
// publishes the CRD schemas there, and every authenticated user may read it (system:discovery),
// so users bound only to the clabernetes lab roles can run the check. Reading the CRD itself
// would need cluster-scoped RBAC.
const c9sOpenAPIPath = "/openapi/v3/apis/c9s.run/v1alpha1"

// supportCheck runs the clabernetes version check once per runtime instance. A cluster that
// lacks the contract is remembered as such; a failed request is retried on the next call.
type supportCheck struct {
	// fetch returns the OpenAPI v3 document of the c9s.run/v1alpha1 group; nil reads it from
	// the API server.
	fetch func(ctx context.Context, r *Runtime) ([]byte, error)

	mu   sync.Mutex
	done bool
	err  error
}

// unsupportedClabernetesError reports a cluster whose clabernetes release lacks the contract the
// runtime needs.
type unsupportedClabernetesError struct {
	reason string
}

func (e *unsupportedClabernetesError) Error() string {
	return fmt.Sprintf("the clabernetes release on this cluster is not supported: %s; "+
		"the c9s runtime needs %s", e.reason, requiredClabernetes)
}

func (e *unsupportedClabernetesError) Unwrap() error {
	return clablabruntime.ErrUnsupportedRuntimeVersion
}

func (s *supportCheck) ensure(ctx context.Context, r *Runtime) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return s.err
	}

	fetch := s.fetch
	if fetch == nil {
		fetch = fetchC9sOpenAPI
	}

	doc, err := fetch(ctx, r)
	switch {
	case apierrors.IsNotFound(err):
		s.done = true
		s.err = &unsupportedClabernetesError{
			reason: "the c9s.run/v1alpha1 API is not served (is clabernetes installed?)",
		}

		return s.err
	case err != nil:
		return fmt.Errorf("failed to check the clabernetes release (%s): %w", c9sOpenAPIPath, err)
	}

	if err := checkNodeRunStateSchema(doc); err != nil {
		var unsupported *unsupportedClabernetesError
		if !asUnsupported(err, &unsupported) {
			return fmt.Errorf("failed to check the clabernetes release: %w", err)
		}
		s.err = err
	}
	s.done = true

	return s.err
}

func asUnsupported(err error, target **unsupportedClabernetesError) bool {
	unsupported, ok := err.(*unsupportedClabernetesError) //nolint:errorlint // created here
	if ok {
		*target = unsupported
	}

	return ok
}

func fetchC9sOpenAPI(ctx context.Context, r *Runtime) ([]byte, error) {
	if r.kubeClient == nil {
		return nil, fmt.Errorf("kubernetes client is not configured")
	}
	client := r.kubeClient.Discovery().RESTClient()
	if client == nil {
		return nil, fmt.Errorf("kubernetes discovery client is not configured")
	}

	return client.Get().AbsPath(c9sOpenAPIPath).Do(ctx).Raw()
}

// openAPISchema is the part of an OpenAPI v3 schema the check reads.
type openAPISchema struct {
	Ref        string                   `json:"$ref,omitempty"`
	AllOf      []openAPISchema          `json:"allOf,omitempty"`
	Properties map[string]openAPISchema `json:"properties,omitempty"`
	Enum       []any                    `json:"enum,omitempty"`
	GVK        []struct {
		Group   string `json:"group"`
		Version string `json:"version"`
		Kind    string `json:"kind"`
	} `json:"x-kubernetes-group-version-kind,omitempty"`
}

type openAPIDocument struct {
	Components struct {
		Schemas map[string]openAPISchema `json:"schemas"`
	} `json:"components"`
}

// checkNodeRunStateSchema reports whether the published Node schema has spec.runState with the
// Running and Stopped states. It returns an *unsupportedClabernetesError when it does not.
func checkNodeRunStateSchema(raw []byte) error {
	var doc openAPIDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("failed to parse the c9s.run/v1alpha1 OpenAPI document: %w", err)
	}

	var node *openAPISchema
	for name := range doc.Components.Schemas {
		schema := doc.Components.Schemas[name]
		for _, gvk := range schema.GVK {
			if gvk.Group == "c9s.run" && gvk.Version == "v1alpha1" && gvk.Kind == "Node" {
				node = &schema
			}
		}
	}
	if node == nil {
		return &unsupportedClabernetesError{reason: "the c9s.run/v1alpha1 API has no Node resource"}
	}

	spec, ok := resolveOpenAPIProperty(doc, *node, "spec")
	if !ok {
		return &unsupportedClabernetesError{reason: "the Node resource has no spec"}
	}
	runState, ok := resolveOpenAPIProperty(doc, spec, "runState")
	if !ok {
		return &unsupportedClabernetesError{
			reason: "nodes.c9s.run has no spec.runState (Node run state)",
		}
	}

	states := map[string]bool{}
	for _, value := range runState.Enum {
		if text, isString := value.(string); isString {
			states[text] = true
		}
	}
	if len(runState.Enum) != 0 && (!states[nodeRunStateRunning] || !states[nodeRunStateStopped]) {
		return &unsupportedClabernetesError{
			reason: fmt.Sprintf("nodes.c9s.run spec.runState does not accept %s and %s",
				nodeRunStateRunning, nodeRunStateStopped),
		}
	}

	return nil
}

// resolveOpenAPIProperty returns a property of a schema, following $ref and allOf.
func resolveOpenAPIProperty(
	doc openAPIDocument,
	schema openAPISchema,
	name string,
) (openAPISchema, bool) {
	for range 8 {
		if property, ok := schema.Properties[name]; ok {
			return resolveOpenAPIRef(doc, property), true
		}
		switch {
		case schema.Ref != "":
			schema = resolveOpenAPIRef(doc, schema)
		case len(schema.AllOf) == 1:
			schema = resolveOpenAPIRef(doc, schema.AllOf[0])
		default:
			return openAPISchema{}, false
		}
	}

	return openAPISchema{}, false
}

func resolveOpenAPIRef(doc openAPIDocument, schema openAPISchema) openAPISchema {
	for range 8 {
		switch {
		case schema.Ref != "":
			name := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
			resolved, ok := doc.Components.Schemas[name]
			if !ok {
				return schema
			}
			schema = resolved
		case len(schema.AllOf) == 1 && len(schema.Properties) == 0:
			schema = schema.AllOf[0]
		default:
			return schema
		}
	}

	return schema
}

// CheckSupport checks that the cluster runs a clabernetes release with the contract the c9s
// runtime needs, and returns an error wrapping labruntime.ErrUnsupportedRuntimeVersion that
// names the required release when it does not. Every cluster operation runs the same check
// once per runtime instance; servers call it at startup to fail early.
func (r *Runtime) CheckSupport(ctx context.Context, identity clablabruntime.Identity) error {
	_, err := r.connect(ctx, identity)

	return err
}
