//go:generate stringer -type status

// Package kmod provides functions to load and unload Linux kernel modules.
//
// Module dependencies are loaded / unloaded automatically according to <mod_dir>/modules.dep.
// Compressed module files can be loaded via a custom InitFunc provided by the caller.
// See SetInitFunc and cmd/modprobe for details.
//
package kmod

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// ErrModuleNotFound is the error resulting if a module can't be found.
var ErrModuleNotFound = errors.New("module not found")

// ErrModuleInUse is the error resulting if a module can't be unloaded because
// it is in use.
var ErrModuleInUse = errors.New("module is in use")

// InitFunc provides a hook to load a kernel module into the kernel.
type InitFunc func(filename string, params string, flags int) error

// Kmod represents internal configuration
type Kmod struct {
	dryrun        bool
	ignoreAlias   bool
	ignoreBuiltin bool
	ignoreStatus  bool
	modConfig     string
	modDir        string
	modInitFunc   InitFunc
	modRootdir    string
	verbose       bool
}

// Option configures Kmod.
type Option func(*Kmod)

// SetDryrun returns an Option that specifies to do everything but actually load or unload.
func SetDryrun() Option { return func(k *Kmod) { k.dryrun = true } }

// SetIgnoreAlias returns an Option that specifies not to consult modules.alias
// to resolve aliases.
func SetIgnoreAlias() Option { return func(k *Kmod) { k.ignoreAlias = true } }

// SetIgnoreBuiltin returns an Option that specifies not to consult modules.builtin
// to find built-in modules.
func SetIgnoreBuiltin() Option { return func(k *Kmod) { k.ignoreBuiltin = true } }

// SetIgnoreStatus returns an Option that specifies not to consult /proc/modules
// to get the current status of a module.
func SetIgnoreStatus() Option { return func(k *Kmod) { k.ignoreStatus = true } }

// SetInitFunc returns an Option that sets fn to be used for loading module files
// into the kernel. The default function tries to use finit_module(2) first and if that
// failes init_module(2). To support compressed module files see the example cmd/modprobe.
func SetInitFunc(fn InitFunc) Option {
	return func(k *Kmod) { k.modInitFunc = fn }
}

// SetConfigFile returns an Option that specifies the (optional) configuration file
// for modules, default: /etc/modprobe.conf. The config file is used only for module
// parameters.
// options <name> parameter [parameter]...
func SetConfigFile(path string) Option {
	return func(k *Kmod) { k.modConfig = path }
}

// SetRootDir returns an Option that sets dir as root directory for modules,
// default: /lib/modules
func SetRootDir(dir string) Option { return func(k *Kmod) { k.modRootdir = dir } }

// SetVerbose returns an Option that specifies to log info messages about what's going on.
func SetVerbose() Option { return func(k *Kmod) { k.verbose = true } }

// New returns a new Kmod.
func New(opts ...Option) (*Kmod, error) {
	k := &Kmod{
		modConfig:  "/etc/modprobe.conf",
		modRootdir: "/lib/modules",
	}
	for _, opt := range opts {
		opt(k)
	}

	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return nil, err
	}
	rel := string(u.Release[:bytes.IndexByte(u.Release[:], 0)])
	k.modDir = filepath.Join(k.modRootdir, rel)

	if _, err := os.Stat(filepath.Join(k.modDir, "modules.dep")); err != nil {
		return nil, err
	}
	return k, nil
}

// Load loads a kernel module. If the module depends on other modules
// Load will try to load all dependencies first.
func (k *Kmod) Load(name, params string, flags int) error {
	name = cleanName(name)

	realname, err := k.checkAlias(name)
	if err != nil {
		return fmt.Errorf("get alias %s failed: %w", name, err)
	}
	if realname != "" {
		k.infof("%s is alias for %s", name, realname)
		name = realname
	}

	builtin, err := k.isBuiltin(name)
	if err != nil {
		return fmt.Errorf("check builtin %s failed: %w", name, err)
	}
	if builtin {
		k.infof("%s is builtin", name)
		return nil
	}

	modules, err := k.modDeps(name)
	if err != nil {
		return err
	}

	if err := k.applyConfig(modules); err != nil {
		return err
	}
	modules[0].params += " " + params
	modules[0].flags = flags

	// load dependencies first, ignore errors
	for i := len(modules) - 1; i > 0; i-- {
		_ = k.load(modules[i])
	}

	// load target module, check error
	err = k.load(modules[0])
	if errors.Is(err, unix.EEXIST) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load %s failed: %w", modules[0].name, err)
	}
	return nil
}

// Unload unloads a module from the kernel. Unload also tries to unload all
// module dependencies that are no longer in use.
func (k *Kmod) Unload(name string) error {
	name = cleanName(name)

	alias, err := k.checkAlias(name)
	if err != nil {
		return fmt.Errorf("check alias %s failed: %w", name, err)
	}
	if alias != "" {
		k.infof("%s is alias for %s", name, alias)
		name = alias
	}

	builtin, err := k.isBuiltin(name)
	if err != nil {
		return fmt.Errorf("check builtin %s failed: %w", name, err)
	}
	if builtin {
		k.infof("%s is builtin", name)
		return nil
	}

	modules, err := k.modDeps(name)
	if err != nil {
		return err
	}

	// unload target module, check error
	if err := k.unload(modules[0]); err != nil {
		if err == unix.EBUSY || err == unix.EAGAIN {
			return ErrModuleInUse
		}
		if err != unix.ENOENT {
			return err
		}
	}

	// unload dependencies, ignore errors
	for i := 1; i < len(modules); i++ {
		_ = k.unload(modules[i])
	}

	return nil
}

// Dependencies returns a list of module dependencies.
func (k *Kmod) Dependencies(name string) ([]string, error) {
	name = cleanName(name)

	realname, err := k.checkAlias(name)
	if err != nil {
		return nil, fmt.Errorf("get alias %s failed: %w", name, err)
	}
	if realname != "" {
		k.infof("%s is alias for %s", name, realname)
		name = realname
	}

	builtin, err := k.isBuiltin(name)
	if err != nil {
		return nil, fmt.Errorf("check builtin %s failed: %w", name, err)
	}
	if builtin {
		return nil, fmt.Errorf("%s is builtin", name)
	}

	modules, err := k.modDeps(name)
	if err != nil {
		return nil, err
	}

	var list []string
	for i := len(modules) - 1; i >= 0; i-- {
		list = append(list, modules[i].path)
	}

	return list, nil
}

func (k *Kmod) isBuiltin(name string) (bool, error) {
	if k.ignoreBuiltin {
		return false, nil
	}
	f, err := os.Open(filepath.Join(k.modDir, "modules.builtin"))
	if err != nil {
		return false, err
	}
	defer f.Close()

	var found bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if pathToName(line) == name {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return found, nil
}

type module struct {
	name   string
	path   string
	params string
	flags  int
}

type status int

const (
	unknown status = iota
	unloaded
	unloading
	loading
	live
	inuse
)

// /proc/modules
//      name | memory size | reference count | references | state: <Live|Loading|Unloading>
// 		macvlan 28672 1 macvtap, Live 0x0000000000000000
func (k *Kmod) modStatus(name string) (status, error) {
	var state status = unknown
	if k.ignoreStatus {
		return state, nil
	}
	f, err := os.Open("/proc/modules")
	if err != nil {
		return state, err
	}
	defer f.Close()

	state = unloaded

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if fields[0] == name {
			if fields[2] != "0" {
				state = inuse
				break
			}
			switch fields[4] {
			case "Live":
				state = live
			case "Loading":
				state = loading
			case "Unloading":
				state = unloading
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return state, err
	}

	return state, nil
}

// modules.alias
//    alias   fs-xfs     xfs
func (k *Kmod) checkAlias(name string) (string, error) {
	if k.ignoreAlias {
		return "", nil
	}
	f, err := os.Open(filepath.Join(k.modDir, "modules.alias"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	var realname string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			if cleanName(fields[1]) == name {
				realname = fields[2]
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return realname, nil
}

// modDeps returns a module and all its depenencies
func (k *Kmod) modDeps(name string) ([]module, error) {
	f, err := os.Open(filepath.Join(k.modDir, "modules.dep"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if pathToName(strings.TrimSuffix(fields[0], ":")) == name {
			deps = fields
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(deps) == 0 {
		return nil, ErrModuleNotFound
	}
	deps[0] = strings.TrimSuffix(deps[0], ":")

	var modules []module
	for _, v := range deps {
		modules = append(modules, module{
			name: pathToName(v),
			path: filepath.Join(k.modDir, v),
		})
	}

	return modules, nil
}

// modprobe.conf
// 	 options <modname> option [option]...
func (k *Kmod) applyConfig(modules []module) error {
	if k.modConfig == "" {
		return nil
	}
	f, err := os.Open(k.modConfig)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "options" {
			continue
		}
		name := cleanName(fields[1])
		for i, m := range modules {
			if m.name == name {
				m.params = strings.Join(fields[2:], " ")
				modules[i] = m
				break
			}
		}
	}
	return scanner.Err()
}

func (k *Kmod) load(m module) error {
	state, err := k.modStatus(m.name)
	if err != nil {
		return err
	}
	if state >= loading {
		return nil
	}
	if k.dryrun {
		return nil
	}
	k.infof("loading %s %s %s", m.name, m.path, m.params)

	if k.modInitFunc != nil {
		return k.modInitFunc(m.path, m.params, m.flags)
	}

	f, err := os.Open(m.path)
	if err != nil {
		return err
	}
	defer f.Close()

	// first try finit_module(2), then init_module(2)
	err = unix.FinitModule(int(f.Fd()), m.params, m.flags)
	if errors.Is(err, unix.ENOSYS) {
		if m.flags != 0 {
			return err
		}
		buf, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		return unix.InitModule(buf, m.params)
	}
	return err
}

func (k *Kmod) unload(m module) error {
	state, err := k.modStatus(m.name)
	if err != nil {
		return err
	}
	if state == unloading || state == unloaded {
		return nil
	}
	if state == inuse {
		return ErrModuleInUse
	}
	if k.dryrun {
		return nil
	}
	k.infof("unloading %s", m.name)
	return unix.DeleteModule(m.name, 0)
}

func (k *Kmod) infof(format string, a ...interface{}) {
	if k.verbose {
		log.Printf(format, a...)
	}
}

func cleanName(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "-", "_")
}

func pathToName(s string) string {
	s = filepath.Base(s)
	for ext := filepath.Ext(s); ext != ""; ext = filepath.Ext(s) {
		s = strings.TrimSuffix(s, ext)
	}
	return cleanName(s)
}
