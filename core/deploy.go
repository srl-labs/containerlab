package core

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	clabcert "github.com/srl-labs/containerlab/cert"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// Deploy the given topology.
// skipcq: GO-R1005
func (c *CLab) Deploy( //nolint: funlen
	ctx context.Context,
	options *DeployOptions,
) ([]clabruntime.GenericContainer, error) {
	var err error

	err = c.ResolveLinks()
	if err != nil {
		return nil, err
	}

	log.Debugf("lab Conf: %+v", c.Config)

	if options.reconfigure {
		_ = c.destroy(ctx, uint(len(c.Nodes)), true)
		log.Info("Removing directory", "path", c.TopoPaths.TopologyLabDir())

		if err := os.RemoveAll(c.TopoPaths.TopologyLabDir()); err != nil {
			return nil, err
		}
	}

	// create management network or use existing one
	if err := c.CreateNetwork(ctx); err != nil {
		return nil, err
	}

	err = clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	if err := c.checkTopologyDefinition(ctx); err != nil {
		return nil, err
	}

	if err := c.loadKernelModules(); err != nil {
		return nil, err
	}

	log.Info("Creating lab directory", "path", c.TopoPaths.TopologyLabDir())
	clabutils.CreateDirectory(c.TopoPaths.TopologyLabDir(), clabconstants.PermissionsDirDefault)

	if !options.skipLabDirFileACLs {
		// adjust ACL for Labdir such that SUDO_UID Users will
		// also have access to lab directory files
		err = clabutils.AdjustFileACLs(c.TopoPaths.TopologyLabDir())
		if err != nil {
			log.Infof("unable to adjust Labdir file ACLs: %v", err)
		}
	}

	// create an empty ansible inventory file that will get populated later
	// we create it here first, so that bind mounts of ansible-inventory.yml file could work
	ansibleInvFPath := c.TopoPaths.AnsibleInventoryFileAbsPath()

	_, err = os.Create(ansibleInvFPath)
	if err != nil {
		return nil, err
	}

	// create an empty nornir simple inventory file that will get populated later
	// we create it here first, so that bind mounts of nornir-simple-inventory.yml file could work
	nornirSimpleInvFPath := c.TopoPaths.NornirSimpleInventoryFileAbsPath()

	_, err = os.Create(nornirSimpleInvFPath)
	if err != nil {
		return nil, err
	}

	// in an similar fashion, create an empty topology data file
	topoDataFPath := c.TopoPaths.TopoExportFile()

	topoDataF, err := os.Create(topoDataFPath)
	if err != nil {
		return nil, err
	}

	if err := c.certificateAuthoritySetup(); err != nil {
		return nil, err
	}

	c.SSHPubKeys, err = c.RetrieveSSHPubKeys()
	if err != nil {
		log.Warn(err)
	}

	if err := c.createAuthzKeysFile(); err != nil {
		return nil, err
	}

	// extraHosts holds host entries for nodes with static IPv4/6 addresses
	// these entries will be used by container runtime to populate /etc/hosts file
	extraHosts := make([]string, 0, len(c.Nodes))

	for _, n := range c.Nodes {
		if n.Config().MgmtIPv4Address != "" {
			log.Debugf("Adding static ipv4 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv4Address)
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv4Address)
		}

		if n.Config().MgmtIPv6Address != "" {
			log.Debugf("Adding static ipv6 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv6Address)
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv6Address)
		}
	}

	for _, n := range c.Nodes {
		n.Config().ExtraHosts = extraHosts
	}

	nodesWg, execCollection, err := c.createNodes(ctx, options.maxWorkers, options.skipPostDeploy)
	if err != nil {
		return nil, err
	}

	// also call deploy on the special nodes endpoints (only host is required for the
	// vxlan stitched endpoints)
	eps := c.getSpecialLinkNodes()["host"].GetEndpoints()
	for _, ep := range eps {
		err = ep.Deploy(ctx)
		if err != nil {
			log.Warnf("failed deploying endpoint %s", ep)
		}
	}

	if nodesWg != nil {
		nodesWg.Wait()
	}

	execCollection.Log()

	if err := c.GenerateInventories(); err != nil {
		return nil, err
	}

	if err := c.GenerateExports(ctx, topoDataF, options.exportTemplate); err != nil {
		return nil, err
	}

	// generate graph of the lab topology
	if options.graph {
		if err = c.GenerateDotGraph(); err != nil {
			log.Error(err)
		}
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("Adding host entries", "path", "/etc/hosts")

	err = c.appendHostsFileEntries(ctx)
	if err != nil {
		log.Errorf("failed to create hosts file: %v", err)
	}

	log.Info("Adding SSH config for nodes", "path", c.TopoPaths.SSHConfigPath())

	err = c.addSSHConfig()
	if err != nil {
		log.Errorf("failed to create ssh config file: %v", err)
	}

	return containers, nil
}

// certificateAuthoritySetup sets up the certificate authority parameters.
func (c *CLab) certificateAuthoritySetup() error {
	// init the Cert storage and CA
	c.Cert.CertStorage = clabcert.NewLocalDirCertStorage(c.TopoPaths)
	c.Cert.CA = clabcert.NewCA()

	s := c.Config.Settings

	// Set defaults for the CA parameters
	keySize := 2048
	validityDuration := time.Until(time.Now().AddDate(1, 0, 0)) // 1 year as default

	// check that Settings.CertificateAuthority exists.
	if s != nil && s.CertificateAuthority != nil {
		// if ValidityDuration is set use the value
		if s.CertificateAuthority.ValidityDuration != 0 {
			validityDuration = s.CertificateAuthority.ValidityDuration
		}

		// if KeyLength is set use the value
		if s.CertificateAuthority.KeySize != 0 {
			keySize = s.CertificateAuthority.KeySize
		}

		// if external CA cert and key are set, propagate to topopaths
		extCACert := s.CertificateAuthority.Cert
		extCAKey := s.CertificateAuthority.Key

		// override external ca and key from env vars
		if v := os.Getenv("CLAB_CA_KEY_FILE"); v != "" {
			extCAKey = v
		}

		if v := os.Getenv("CLAB_CA_CERT_FILE"); v != "" {
			extCACert = v
		}

		if extCACert != "" && extCAKey != "" {
			err := c.TopoPaths.SetExternalCaFiles(extCACert, extCAKey)
			if err != nil {
				return err
			}
		}
	}

	// define the attributes used to generate the CA Cert
	caCertInput := &clabcert.CACSRInput{
		CommonName:   c.Config.Name + " lab CA",
		Country:      "US",
		Expiry:       validityDuration,
		Organization: "containerlab",
		KeySize:      keySize,
	}

	return c.LoadOrGenerateCA(caCertInput)
}

// createNodes schedules nodes creation and returns a waitgroup for all nodes
// with the exec collection created from the exec config of each node.
// The exec collection is returned to the caller to ensure that the execution log
// is printed after the nodes are created.
// Nodes interdependencies are created in this function.
func (c *CLab) createNodes(
	ctx context.Context,
	maxWorkers uint,
	skipPostDeploy bool,
) (*sync.WaitGroup, *clabexec.ExecCollection, error) {
	for _, node := range c.Nodes {
		c.dependencyManager.AddNode(node)
	}

	// nodes with static mgmt IP should be scheduled before the dynamic ones
	err := c.createStaticDynamicDependency()
	if err != nil {
		return nil, nil, err
	}

	// create user-defined node dependencies done with `wait-for` property of the deployment stage
	err = c.createWaitForDependency()
	if err != nil {
		return nil, nil, err
	}

	// create a set of dependencies, that makes the ignite nodes start one after the other
	err = c.createIgniteSerialDependency()
	if err != nil {
		return nil, nil, err
	}

	// make network namespace shared containers start in the right order
	c.createNamespaceSharingDependency()

	// Add possible additional dependencies here

	// make sure that there are no unresolvable dependencies, which would deadlock.
	err = c.dependencyManager.CheckAcyclicity()
	if err != nil {
		return nil, nil, err
	}

	// start scheduling
	NodesWg, execCollection := c.scheduleNodes(ctx, int(maxWorkers), skipPostDeploy)

	return NodesWg, execCollection, nil
}
