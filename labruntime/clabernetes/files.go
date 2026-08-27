package clabernetes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	fileModeRead    = "read"
	fileModeExecute = "execute"
	// An inline startup-config is by definition a partial configuration merged over the kind
	// default; the .partial marker in the staged path is how node implementations detect that
	// (same convention as clabverter).
	inlineStartupConfigMountPath = "/clabernetes/startup-config.partial.cfg"
	maxConfigMapFileBytes        = 950_000
	kubernetesNameMaxLen         = 63
	clabernetesNamingNonPrefixed = "non-prefixed"

	clabDirVar     = "__clabDir__"
	clabLabNameVar = "__clabLabName__"
	nodeDirVar     = "__clabNodeDir__"
	nodeNameVar    = "__clabNodeName__"
)

var (
	invalidDNSLabelChars = regexp.MustCompile(`[^a-z0-9\-]`) //nolint:gochecknoglobals
	startsWithNonAlpha   = regexp.MustCompile(`^[^a-z]`)     //nolint:gochecknoglobals
	endsWithNonAlpha     = regexp.MustCompile(`[^a-z]$`)     //nolint:gochecknoglobals
)

type clabRuntimeConfig struct {
	Name     string              `yaml:"name,omitempty"`
	Prefix   *string             `yaml:"prefix,omitempty"`
	Mgmt     *clabtypes.MgmtNet  `yaml:"mgmt,omitempty"`
	Settings *clabtypes.Settings `yaml:"settings,omitempty"`
	Topology *clabtypes.Topology `yaml:"topology,omitempty"`
}

type clabernetesRenderConfig struct {
	Name     string                     `yaml:"name,omitempty"`
	Prefix   *string                    `yaml:"prefix,omitempty"`
	Mgmt     *clabtypes.MgmtNet         `yaml:"mgmt,omitempty"`
	Settings *clabtypes.Settings        `yaml:"settings,omitempty"`
	Topology *clabernetesRenderTopology `yaml:"topology,omitempty"`
}

type clabernetesRenderTopology struct {
	Defaults *clabtypes.NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*clabtypes.NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*clabtypes.NodeDefinition `yaml:"nodes,omitempty"`
	Groups   map[string]*clabtypes.NodeDefinition `yaml:"groups,omitempty"`
	Links    []*clablinks.LinkBriefRaw            `yaml:"links,omitempty"`
}

type stagedConfigMap struct {
	name          string
	nodeName      string
	binaryData    map[string][]byte
	keyByFilePath map[string]string
	mounts        []stagedConfigMapMount
}

type stagedConfigMapMount struct {
	nodeName      string
	filePath      string
	configMapName string
	configMapPath string
	mode          string
}

type stagedLocalFile struct {
	filePath string
	mode     string
	content  []byte
}

func stageTopologyLocalFiles(
	req clablabruntime.DeployRequest,
) ([]byte, []stagedConfigMap, string, error) {
	config := &clabRuntimeConfig{}
	if err := yaml.Unmarshal(req.TopologyDefinition, config); err != nil {
		return nil, nil, "", fmt.Errorf(
			"failed to parse rendered topology for clabernetes preparation: %w",
			err,
		)
	}

	naming := clabernetesNamingMode(config)
	if config.Topology == nil || len(config.Topology.Nodes) == 0 {
		return req.TopologyDefinition, nil, naming, nil
	}

	extraConfigMaps := map[string]*stagedConfigMap{}
	startupConfigMaps := map[string]*stagedConfigMap{}
	// Always render parsed links through the c9s brief-link boundary so an extended link's
	// behavior does not depend on whether some unrelated field changed the definition.
	definitionChanged := len(config.Topology.Links) > 0

	nodeNames := make([]string, 0, len(config.Topology.Nodes))
	for nodeName := range config.Topology.Nodes {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	if req.TopologyFile != "" {
		topologyFileDir := filepath.Dir(req.TopologyFile)
		topologyLabDir := req.TopologyLabDir
		if topologyLabDir == "" && config.Name != "" {
			topologyLabDir = filepath.Join(topologyFileDir, "clab-"+config.Name)
		}

		for _, nodeName := range nodeNames {
			if err := stageStartupConfig(
				config,
				req.Name,
				nodeName,
				topologyFileDir,
				topologyLabDir,
				startupConfigMaps,
				&definitionChanged,
			); err != nil {
				return nil, nil, "", err
			}

			if err := stageLicenseFile(
				config,
				req.Name,
				nodeName,
				topologyFileDir,
				topologyLabDir,
				extraConfigMaps,
			); err != nil {
				return nil, nil, "", err
			}

			if err := stageBindFiles(
				config,
				req.Name,
				nodeName,
				topologyFileDir,
				topologyLabDir,
				extraConfigMaps,
			); err != nil {
				return nil, nil, "", err
			}

			if err := stageAdditionalNodeFiles(
				config,
				req.Name,
				nodeName,
				topologyFileDir,
				topologyLabDir,
				extraConfigMaps,
			); err != nil {
				return nil, nil, "", err
			}
		}
	}

	topologyDefinition := req.TopologyDefinition
	if definitionChanged {
		updatedDefinition, err := renderClabernetesTopologyDefinition(config)
		if err != nil {
			return nil, nil, "", fmt.Errorf(
				"failed to render updated clabernetes topology definition: %w",
				err,
			)
		}
		topologyDefinition = updatedDefinition
	}

	stagedConfigMaps := collectStagedConfigMaps(startupConfigMaps, extraConfigMaps)

	return topologyDefinition, stagedConfigMaps, naming, nil
}

func renderClabernetesTopologyDefinition(config *clabRuntimeConfig) ([]byte, error) {
	if config == nil {
		return nil, fmt.Errorf("topology config is nil")
	}

	rendered := &clabernetesRenderConfig{
		Name:     config.Name,
		Prefix:   config.Prefix,
		Mgmt:     config.Mgmt,
		Settings: config.Settings,
	}

	if config.Topology != nil {
		links, err := clabernetesBriefLinks(config.Topology.Links)
		if err != nil {
			return nil, err
		}

		rendered.Topology = &clabernetesRenderTopology{
			Defaults: config.Topology.Defaults,
			Kinds:    config.Topology.Kinds,
			Nodes:    config.Topology.Nodes,
			Groups:   config.Topology.Groups,
			Links:    links,
		}
	}

	return yaml.Marshal(rendered)
}

func clabernetesBriefLinks(
	links []*clablinks.LinkDefinition,
) ([]*clablinks.LinkBriefRaw, error) {
	if len(links) == 0 {
		return nil, nil
	}

	briefLinks := make([]*clablinks.LinkBriefRaw, 0, len(links))
	for _, link := range links {
		if link == nil || link.Link == nil {
			continue
		}

		brief, err := clabernetesBriefLink(link)
		if err != nil {
			return nil, err
		}
		briefLinks = append(briefLinks, brief)
	}

	return briefLinks, nil
}

func clabernetesBriefLink(
	link *clablinks.LinkDefinition,
) (*clablinks.LinkBriefRaw, error) {
	switch raw := link.Link.(type) {
	case *clablinks.LinkBriefRaw:
		return raw, nil
	case *clablinks.LinkVEthRaw:
		return raw.ToLinkBriefRaw(), nil
	case *clablinks.LinkHostRaw:
		return raw.ToLinkBriefRaw(), nil
	case *clablinks.LinkMgmtNetRaw:
		return raw.ToLinkBriefRaw(), nil
	case *clablinks.LinkMacVlanRaw:
		return raw.ToLinkBriefRaw(), nil
	default:
		return nil, fmt.Errorf(
			"failed to render clabernetes-compatible brief link for %s link",
			link.Link.GetType(),
		)
	}
}

func clabernetesNamingMode(config *clabRuntimeConfig) string {
	if config == nil || config.Prefix == nil || *config.Prefix != "" {
		return ""
	}

	return clabernetesNamingNonPrefixed
}

func stageStartupConfig(
	config *clabRuntimeConfig,
	topologyName,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
	configMaps map[string]*stagedConfigMap,
	definitionChanged *bool,
) error {
	startupConfig := config.Topology.GetNodeStartupConfig(nodeName)
	if startupConfig == "" {
		return nil
	}

	configMap := getOrCreateStagedConfigMap(
		configMaps,
		nodeName,
		safeKubernetesName(topologyName, nodeName, "startup-config"),
	)

	if strings.Contains(startupConfig, "\n") {
		nodeDefinition := config.Topology.Nodes[nodeName]
		if nodeDefinition == nil {
			nodeDefinition = &clabtypes.NodeDefinition{}
			config.Topology.Nodes[nodeName] = nodeDefinition
		}
		nodeDefinition.StartupConfig = inlineStartupConfigMountPath
		*definitionChanged = true

		return addStagedConfigMapData(
			configMap,
			inlineStartupConfigMountPath,
			"startup-config",
			fileModeRead,
			[]byte(startupConfig),
		)
	}

	files, err := resolveLocalFiles(startupConfig, nodeName, topologyFileDir, topologyLabDir)
	if err != nil {
		return fmt.Errorf("failed staging startup-config for node %q: %w", nodeName, err)
	}

	for _, file := range files {
		if err := addStagedConfigMapData(
			configMap,
			file.filePath,
			"startup-config",
			file.mode,
			file.content,
		); err != nil {
			return err
		}
	}

	return nil
}

func stageLicenseFile(
	config *clabRuntimeConfig,
	topologyName,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
	configMaps map[string]*stagedConfigMap,
) error {
	license := config.Topology.GetNodeLicense(nodeName)
	if license == "" {
		return nil
	}

	configMap := getOrCreateStagedConfigMap(
		configMaps,
		nodeName,
		safeKubernetesName(topologyName, nodeName, "files"),
	)

	return stageSourcePathIntoConfigMap(
		configMap,
		license,
		nodeName,
		topologyFileDir,
		topologyLabDir,
	)
}

func stageBindFiles(
	config *clabRuntimeConfig,
	topologyName,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
	configMaps map[string]*stagedConfigMap,
) error {
	binds, err := config.Topology.GetNodeBinds(nodeName)
	if err != nil {
		return fmt.Errorf("failed parsing bind mounts for node %q: %w", nodeName, err)
	}
	if len(binds) == 0 {
		return nil
	}

	configMap := getOrCreateStagedConfigMap(
		configMaps,
		nodeName,
		safeKubernetesName(topologyName, nodeName, "files"),
	)

	for _, bind := range binds {
		parsedBind, err := clabtypes.NewBindFromString(bind)
		if err != nil {
			return fmt.Errorf("failed parsing bind %q for node %q: %w", bind, nodeName, err)
		}
		if parsedBind.Src() == "" {
			continue
		}

		if err := stageSourcePathIntoConfigMap(
			configMap,
			parsedBind.Src(),
			nodeName,
			topologyFileDir,
			topologyLabDir,
		); err != nil {
			return err
		}
	}

	return nil
}

// stageAdditionalNodeFiles covers path-bearing fields that containerlab normally resolves on
// the machine running the CLI. A device pod cannot see those paths unless the adapter copies
// them into the node's ConfigMap-backed file projection first.
func stageAdditionalNodeFiles(
	config *clabRuntimeConfig,
	topologyName,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
	configMaps map[string]*stagedConfigMap,
) error {
	pathsByField := map[string][]string{
		"env-files": config.Topology.GetNodeEnvFiles(nodeName),
	}

	if extras := config.Topology.GetNodeExtras(nodeName); extras != nil {
		pathsByField["extras.srl-agents"] = extras.SRLAgents
		pathsByField["extras.ceos-copy-to-flash"] = extras.CeosCopyToFlash
	}

	fieldNames := make([]string, 0, len(pathsByField))
	for fieldName := range pathsByField {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		for _, sourcePath := range pathsByField[fieldName] {
			if strings.TrimSpace(sourcePath) == "" {
				continue
			}

			configMap := getOrCreateStagedConfigMap(
				configMaps,
				nodeName,
				safeKubernetesName(topologyName, nodeName, "files"),
			)

			if err := stageSourcePathIntoConfigMap(
				configMap,
				sourcePath,
				nodeName,
				topologyFileDir,
				topologyLabDir,
			); err != nil {
				return fmt.Errorf(
					"failed staging %s path %q for node %q: %w",
					fieldName,
					sourcePath,
					nodeName,
					err,
				)
			}
		}
	}

	return nil
}

func stageSourcePathIntoConfigMap(
	configMap *stagedConfigMap,
	sourcePath,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
) error {
	files, err := resolveLocalFiles(sourcePath, nodeName, topologyFileDir, topologyLabDir)
	if err != nil {
		return fmt.Errorf(
			"failed staging source path %q for node %q: %w",
			sourcePath,
			nodeName,
			err,
		)
	}

	for _, file := range files {
		configMapKey := uniqueConfigMapKey(configMap, file.filePath)
		if err := addStagedConfigMapData(
			configMap,
			file.filePath,
			configMapKey,
			file.mode,
			file.content,
		); err != nil {
			return err
		}
	}

	return nil
}

func resolveLocalFiles(
	sourcePath,
	nodeName,
	topologyFileDir,
	topologyLabDir string,
) ([]stagedLocalFile, error) {
	displayPath := replaceClabPathVariables(sourcePath, nodeName, topologyLabDir)
	resolvedPath := clabutils.ResolvePath(displayPath, topologyFileDir)

	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, err
	}

	if !fileInfo.IsDir() {
		file, err := loadStagedLocalFile(displayPath, resolvedPath, fileInfo)
		if err != nil {
			return nil, err
		}

		return []stagedLocalFile{file}, nil
	}

	files := []stagedLocalFile{}
	err = filepath.WalkDir(resolvedPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(resolvedPath, path)
		if err != nil {
			return err
		}

		file, err := loadStagedLocalFile(
			filepath.ToSlash(filepath.Join(displayPath, relativePath)),
			path,
			fileInfo,
		)
		if err != nil {
			return err
		}

		files = append(files, file)

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].filePath < files[j].filePath
	})

	return files, nil
}

func loadStagedLocalFile(
	filePath,
	resolvedPath string,
	fileInfo os.FileInfo,
) (stagedLocalFile, error) {
	content, err := os.ReadFile(resolvedPath) //nolint:gosec
	if err != nil {
		return stagedLocalFile{}, err
	}
	if len(content) > maxConfigMapFileBytes {
		return stagedLocalFile{}, fmt.Errorf(
			"file %q is %d bytes, larger than the supported ConfigMap file limit of %d bytes",
			resolvedPath,
			len(content),
			maxConfigMapFileBytes,
		)
	}

	mode := fileModeRead
	if fileInfo.Mode()&0o111 != 0 {
		mode = fileModeExecute
	}

	return stagedLocalFile{
		filePath: filepath.ToSlash(filePath),
		mode:     mode,
		content:  content,
	}, nil
}

func replaceClabPathVariables(sourcePath, nodeName, topologyLabDir string) string {
	labName := filepath.Base(topologyLabDir)
	nodeDir := ""
	if topologyLabDir != "" && nodeName != "" {
		nodeDir = filepath.Join(topologyLabDir, nodeName)
	}

	replacer := strings.NewReplacer(
		clabDirVar, topologyLabDir,
		clabLabNameVar, labName,
		nodeDirVar, nodeDir,
		nodeNameVar, nodeName,
	)

	return replacer.Replace(sourcePath)
}

func getOrCreateStagedConfigMap(
	configMaps map[string]*stagedConfigMap,
	nodeName,
	name string,
) *stagedConfigMap {
	configMap, ok := configMaps[nodeName]
	if ok {
		return configMap
	}

	configMap = &stagedConfigMap{
		name:          name,
		nodeName:      nodeName,
		binaryData:    map[string][]byte{},
		keyByFilePath: map[string]string{},
	}
	configMaps[nodeName] = configMap

	return configMap
}

func addStagedConfigMapData(
	configMap *stagedConfigMap,
	filePath,
	configMapKey,
	mode string,
	content []byte,
) error {
	if existingKey, ok := configMap.keyByFilePath[filePath]; ok {
		if !bytes.Equal(configMap.binaryData[existingKey], content) {
			return fmt.Errorf("staged file path %q has conflicting content", filePath)
		}

		return nil
	}

	configMap.binaryData[configMapKey] = content
	configMap.keyByFilePath[filePath] = configMapKey
	configMap.mounts = append(configMap.mounts, stagedConfigMapMount{
		nodeName:      configMap.nodeName,
		filePath:      filePath,
		configMapName: configMap.name,
		configMapPath: configMapKey,
		mode:          mode,
	})

	return nil
}

func uniqueConfigMapKey(configMap *stagedConfigMap, filePath string) string {
	configMapKey := safeConfigMapKey(filePath)
	if _, exists := configMap.binaryData[configMapKey]; !exists {
		return configMapKey
	}

	digest := sha256.Sum256([]byte(filePath))
	for idx := 0; ; idx++ {
		candidate := safeKubernetesName(
			configMapKey,
			hex.EncodeToString(digest[:])[0:7],
			fmt.Sprintf("%d", idx),
		)
		if _, exists := configMap.binaryData[candidate]; !exists {
			return candidate
		}
	}
}

func collectStagedConfigMaps(configMapGroups ...map[string]*stagedConfigMap) []stagedConfigMap {
	configMaps := []stagedConfigMap{}

	for _, configMapGroup := range configMapGroups {
		nodeNames := make([]string, 0, len(configMapGroup))
		for nodeName := range configMapGroup {
			nodeNames = append(nodeNames, nodeName)
		}
		sort.Strings(nodeNames)

		for _, nodeName := range nodeNames {
			configMap := configMapGroup[nodeName]
			sort.Slice(configMap.mounts, func(i, j int) bool {
				return configMap.mounts[i].filePath < configMap.mounts[j].filePath
			})
			configMaps = append(configMaps, *configMap)
		}
	}

	return configMaps
}

func setTopologyFilesFromConfigMaps(
	topology *unstructured.Unstructured,
	configMaps []stagedConfigMap,
) error {
	if len(configMaps) == 0 {
		return nil
	}

	filesFromConfigMap := map[string]any{}
	for _, configMap := range configMaps {
		for _, mount := range configMap.mounts {
			nodeFiles, _ := filesFromConfigMap[mount.nodeName].([]any)
			nodeFiles = append(nodeFiles, map[string]any{
				"filePath":      mount.filePath,
				"configMapName": mount.configMapName,
				"configMapPath": mount.configMapPath,
				"mode":          mount.mode,
			})
			filesFromConfigMap[mount.nodeName] = nodeFiles
		}
	}

	deployment, found, err := unstructured.NestedMap(topology.Object, "spec", "deployment")
	if err != nil {
		return err
	}
	if !found {
		deployment = map[string]any{}
	}

	deployment["filesFromConfigMap"] = filesFromConfigMap

	return unstructured.SetNestedMap(topology.Object, deployment, "spec", "deployment")
}

func (r *Runtime) applyStagedConfigMaps(
	ctx context.Context,
	namespace string,
	topologyName string,
	configMaps []stagedConfigMap,
) error {
	for _, staged := range configMaps {
		configMap := stagedConfigMapObject(namespace, topologyName, staged, nil)

		_, err := r.kubeClient.CoreV1().ConfigMaps(namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			existing, getErr := r.kubeClient.CoreV1().ConfigMaps(namespace).
				Get(ctx, staged.name, metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("failed to get existing staged ConfigMap %s/%s: %w",
					namespace,
					staged.name,
					getErr,
				)
			}

			updated := existing.DeepCopy()
			updated.Labels = mergeDesiredMetadata(existing.Labels, configMap.Labels)
			updated.Data = configMap.Data
			updated.BinaryData = configMap.BinaryData
			err = nil
			if !stagedConfigMapsConform(existing, updated) {
				_, err = r.kubeClient.CoreV1().ConfigMaps(namespace).
					Update(ctx, updated, metav1.UpdateOptions{})
			}
		}
		if err != nil {
			return fmt.Errorf("failed to apply staged ConfigMap %s/%s: %w",
				namespace,
				staged.name,
				err,
			)
		}
	}

	return nil
}

func stagedConfigMapsConform(existing, desired *corev1.ConfigMap) bool {
	return apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(existing.Data, desired.Data) &&
		apiequality.Semantic.DeepEqual(existing.BinaryData, desired.BinaryData)
}

func stagedConfigMapObject(
	namespace string,
	topologyName string,
	staged stagedConfigMap,
	ownerReferences []metav1.OwnerReference,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            staged.name,
			Namespace:       namespace,
			OwnerReferences: ownerReferences,
			Labels: map[string]string{
				labelApp:           clabernetesAppValue,
				labelTopologyOwner: topologyName,
				labelTopologyNode:  staged.nodeName,
			},
		},
		BinaryData: staged.binaryData,
	}
}

func (r *Runtime) setStagedConfigMapNodeOwnerReferences(
	ctx context.Context,
	namespace string,
	configMaps []stagedConfigMap,
	nodes map[string]*unstructured.Unstructured,
) error {
	if len(configMaps) == 0 {
		return nil
	}

	for _, staged := range configMaps {
		node := nodes[staged.nodeName]
		if node == nil {
			return fmt.Errorf("failed to find c9s node %s/%s for staged ConfigMap ownership",
				namespace, staged.nodeName)
		}
		ownerReferences := []metav1.OwnerReference{
			{
				APIVersion: c9sAPIVersion,
				Kind:       "Node",
				Name:       node.GetName(),
				UID:        node.GetUID(),
			},
		}

		configMap, err := r.kubeClient.CoreV1().ConfigMaps(namespace).
			Get(ctx, staged.name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			configMap = stagedConfigMapObject(
				namespace,
				node.GetLabels()[labelTopologyOwner],
				staged,
				ownerReferences,
			)
			if _, err = r.kubeClient.CoreV1().ConfigMaps(namespace).
				Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf(
					"failed to recreate staged ConfigMap %s/%s with owner references: %w",
					namespace,
					staged.name,
					err,
				)
			}

			continue
		}
		if err != nil {
			return fmt.Errorf("failed to get staged ConfigMap %s/%s for owner update: %w",
				namespace,
				staged.name,
				err,
			)
		}

		if apiequality.Semantic.DeepEqual(configMap.OwnerReferences, ownerReferences) {
			continue
		}
		configMap.OwnerReferences = ownerReferences

		if _, err = r.kubeClient.CoreV1().ConfigMaps(namespace).
			Update(ctx, configMap, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update staged ConfigMap %s/%s owner references: %w",
				namespace,
				staged.name,
				err,
			)
		}
	}

	return nil
}

func (r *Runtime) deleteStagedConfigMaps(
	ctx context.Context,
	namespace string,
	configMaps []stagedConfigMap,
) {
	for _, staged := range configMaps {
		err := r.kubeClient.CoreV1().ConfigMaps(namespace).
			Delete(ctx, staged.name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			log.Debug("failed to delete staged clabernetes ConfigMap",
				"namespace", namespace,
				"name", staged.name,
				"error", err,
			)
		}
	}
}

func (r *Runtime) deleteStaleConfigMaps(
	ctx context.Context,
	namespace,
	topologyName string,
	desired []stagedConfigMap,
) error {
	desiredNames := make(map[string]struct{}, len(desired))
	for _, staged := range desired {
		desiredNames[staged.name] = struct{}{}
	}

	configMaps, err := r.kubeClient.CoreV1().ConfigMaps(namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labelTopologyOwner + "=" + topologyName},
	)
	if err != nil {
		return fmt.Errorf("failed to list staged ConfigMaps for c9s lab %s/%s: %w",
			namespace, topologyName, err)
	}

	for idx := range configMaps.Items {
		name := configMaps.Items[idx].Name
		if _, keep := desiredNames[name]; keep {
			continue
		}
		if err := r.kubeClient.CoreV1().ConfigMaps(namespace).
			Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale staged ConfigMap %s/%s: %w",
				namespace, name, err)
		}
	}

	return nil
}

func safeConfigMapKey(filePath string) string {
	parts := strings.FieldsFunc(filepath.ToSlash(filePath), func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return "file"
	}

	return safeKubernetesName(parts...)
}

func safeKubernetesName(parts ...string) string {
	name := strings.Join(parts, "-")
	if len(name) > kubernetesNameMaxLen {
		digest := sha256.Sum256([]byte(name))
		name = name[0:kubernetesNameMaxLen-8] + "-" + hex.EncodeToString(digest[:])[0:7]
	}

	name = strings.ToLower(name)
	name = invalidDNSLabelChars.ReplaceAllString(name, "-")
	name = startsWithNonAlpha.ReplaceAllString(name, "z")
	name = endsWithNonAlpha.ReplaceAllString(name, "z")

	return name
}
