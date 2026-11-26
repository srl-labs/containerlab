package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

func TestRootRequirementHelpers(t *testing.T) {
	tests := []struct {
		name               string
		runtime            string
		wantGlobalRoot     bool
		wantCommandSkipped bool
	}{
		{
			name:               "default runtime",
			runtime:            "",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "docker runtime",
			runtime:            "docker",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "podman runtime",
			runtime:            "podman",
			wantGlobalRoot:     true,
			wantCommandSkipped: false,
		},
		{
			name:               "clabernetes runtime",
			runtime:            clablabruntime.ClabernetesRuntimeName,
			wantGlobalRoot:     false,
			wantCommandSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := globalRuntimeRequiresRoot(tt.runtime); got != tt.wantGlobalRoot {
				t.Fatalf("globalRuntimeRequiresRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantGlobalRoot)
			}

			if got := commandSkipsRoot(tt.runtime); got != tt.wantCommandSkipped {
				t.Fatalf("commandSkipsRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantCommandSkipped)
			}
		})
	}
}

func TestNamespaceFlagHasNoShorthand(t *testing.T) {
	optionsInstance = nil
	t.Cleanup(func() { optionsInstance = nil })

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	flag := root.PersistentFlags().Lookup("namespace")
	if flag == nil {
		t.Fatal("--namespace flag was not registered")
	}
	if flag.Shorthand != "" {
		t.Fatalf("--namespace shorthand = %q, want none", flag.Shorthand)
	}
	if err := root.PersistentFlags().Set("namespace", "default"); err != nil {
		t.Fatal(err)
	}
	if got := GetOptions().Global.Namespace; got != "default" {
		t.Fatalf("namespace option = %q, want default", got)
	}
}

func TestApplyLabRuntimeDefaultTimeout(t *testing.T) {
	tests := []struct {
		name            string
		runtime         string
		explicitFlag    string
		explicitEnv     string
		initialTimeout  time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "clabernetes default",
			runtime:         clablabruntime.ClabernetesRuntimeName,
			initialTimeout:  defaultTimeout,
			expectedTimeout: defaultLabRuntimeTimeout,
		},
		{
			name:            "docker default unchanged",
			runtime:         "docker",
			initialTimeout:  defaultTimeout,
			expectedTimeout: defaultTimeout,
		},
		{
			name:            "explicit flag wins",
			runtime:         clablabruntime.ClabernetesRuntimeName,
			explicitFlag:    "45s",
			initialTimeout:  defaultTimeout,
			expectedTimeout: 45 * time.Second,
		},
		{
			name:            "explicit environment wins",
			runtime:         clablabruntime.ClabernetesRuntimeName,
			explicitEnv:     "3m",
			initialTimeout:  3 * time.Minute,
			expectedTimeout: 3 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalTimeout, timeoutWasSet := os.LookupEnv("CLAB_TIMEOUT")
			if tt.explicitEnv == "" {
				if err := os.Unsetenv("CLAB_TIMEOUT"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("CLAB_TIMEOUT", tt.explicitEnv)
			}
			t.Cleanup(func() {
				if timeoutWasSet {
					_ = os.Setenv("CLAB_TIMEOUT", originalTimeout)
				} else {
					_ = os.Unsetenv("CLAB_TIMEOUT")
				}
			})

			root := &cobra.Command{Use: "containerlab"}
			options := &Options{Global: &GlobalOptions{
				Runtime: tt.runtime,
				Timeout: tt.initialTimeout,
			}}
			root.PersistentFlags().DurationVar(
				&options.Global.Timeout,
				"timeout",
				options.Global.Timeout,
				"timeout",
			)
			deploy := &cobra.Command{Use: "deploy"}
			root.AddCommand(deploy)
			if tt.explicitFlag != "" {
				if err := root.PersistentFlags().Set("timeout", tt.explicitFlag); err != nil {
					t.Fatal(err)
				}
			}

			applyLabRuntimeDefaultTimeout(deploy, options)
			if options.Global.Timeout != tt.expectedTimeout {
				t.Fatalf(
					"timeout = %s, want %s",
					options.Global.Timeout,
					tt.expectedTimeout,
				)
			}
		})
	}
}

func TestCheckLabRuntimeCommandSupport(t *testing.T) {
	root := &cobra.Command{Use: "containerlab"}
	graph := &cobra.Command{Use: "graph"}
	tools := &cobra.Command{Use: "tools"}
	sshx := &cobra.Command{Use: "sshx"}
	netem := &cobra.Command{Use: "netem"}
	netemSet := &cobra.Command{Use: "set"}
	deploy := &cobra.Command{Use: "deploy"}

	root.AddCommand(graph, tools, deploy)
	tools.AddCommand(sshx, netem)
	netem.AddCommand(netemSet)

	tests := []struct {
		name    string
		runtime string
		cmd     *cobra.Command
		wantErr bool
	}{
		{
			name:    "graph with docker runtime",
			runtime: "docker",
			cmd:     graph,
			wantErr: false,
		},
		{
			name:    "deploy with clabernetes runtime",
			runtime: clablabruntime.ClabernetesRuntimeName,
			cmd:     deploy,
			wantErr: false,
		},
		{
			name:    "graph with clabernetes runtime",
			runtime: clablabruntime.ClabernetesRuntimeName,
			cmd:     graph,
			wantErr: true,
		},
		{
			name:    "tools subcommand with clabernetes runtime",
			runtime: clablabruntime.ClabernetesRuntimeName,
			cmd:     sshx,
			wantErr: true,
		},
		{
			name:    "tools netem with clabernetes runtime",
			runtime: clablabruntime.ClabernetesRuntimeName,
			cmd:     netemSet,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkLabRuntimeCommandSupport(tt.cmd, tt.runtime)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkLabRuntimeCommandSupport(%q, %q) error = %v, wantErr %v",
					tt.cmd.Name(), tt.runtime, err, tt.wantErr)
			}
		})
	}
}

func TestCheckLabRuntimeFlagAndNestedCommandSupport(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		flag      string
		flagValue string
		wantErr   bool
	}{
		{name: "deploy dry-run supported", command: "deploy", flag: "dry-run", flagValue: "true"},
		{
			name:      "deploy worker controls supported",
			command:   "deploy",
			flag:      "max-workers",
			flagValue: "4",
		},
		{
			name:      "deploy node filter rejected",
			command:   "deploy",
			flag:      "node-filter",
			flagValue: "n1",
			wantErr:   true,
		},
		{
			name:      "destroy graceful rejected",
			command:   "destroy",
			flag:      "graceful",
			flagValue: "true",
			wantErr:   true,
		},
		{
			name:      "destroy worker controls supported",
			command:   "destroy",
			flag:      "max-workers",
			flagValue: "4",
		},
		{
			name:      "destroy node filter rejected",
			command:   "destroy",
			flag:      "node-filter",
			flagValue: "n1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v = nil
			t.Cleanup(func() { v = nil })

			root := &cobra.Command{Use: "containerlab"}
			command := &cobra.Command{Use: tt.command}
			root.AddCommand(command)
			switch tt.flag {
			case "dry-run", "graceful":
				command.Flags().Bool(tt.flag, false, "")
			case "max-workers":
				command.Flags().Uint(tt.flag, 0, "")
			case "node-filter":
				command.Flags().StringSlice(tt.flag, nil, "")
			}
			if err := command.Flags().Set(tt.flag, tt.flagValue); err != nil {
				t.Fatal(err)
			}

			err := checkLabRuntimeCommandSupport(command, clablabruntime.ClabernetesRuntimeName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkLabRuntimeCommandSupport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("inspect interfaces and capture accepted", func(t *testing.T) {
		root := &cobra.Command{Use: "containerlab"}
		inspect := &cobra.Command{Use: "inspect"}
		interfaces := &cobra.Command{Use: "interfaces"}
		capture := &cobra.Command{Use: "capture"}
		root.AddCommand(inspect, capture)
		inspect.AddCommand(interfaces)

		for _, cmd := range []*cobra.Command{interfaces, capture} {
			if err := checkLabRuntimeCommandSupport(
				cmd,
				clablabruntime.ClabernetesRuntimeName,
			); err != nil {
				t.Fatalf("%s rejected for clabernetes: %v", cmd.Name(), err)
			}
		}
	})
}

func TestRuntimeIndependentCommandsExist(t *testing.T) {
	optionsInstance = nil
	t.Cleanup(func() { optionsInstance = nil })

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}
	root.InitDefaultHelpCmd()

	for commandPath := range runtimeIndependentCommands {
		// Cobra adds the completion request commands only when it executes
		if strings.HasPrefix(commandPath, "__") {
			continue
		}

		found, _, err := root.Find(strings.Split(commandPath, "."))
		if err != nil || getCommandPath(found) != commandPath {
			t.Errorf("runtime independent command %q is not a command", commandPath)
		}
	}
}

func TestCommandIsRuntimeIndependent(t *testing.T) {
	tests := []struct {
		args []string
		env  map[string]string
		want bool
	}{
		{args: []string{"help"}, want: true},
		{args: []string{"help", "deploy"}, want: true},
		{args: []string{"version"}, want: true},
		{args: []string{"version", "check"}, want: true},
		{args: []string{"version", "upgrade"}, want: false},
		{args: []string{"completion", "bash"}, want: true},
		{args: []string{"generate", "--name", "lab", "--nodes", "2"}, want: true},
		{args: []string{"tools", "cert", "ca", "create"}, want: true},
		{args: []string{"tools", "cert", "sign"}, want: true},
		{args: []string{"deploy"}, want: false},
		{args: []string{"inspect", "--all"}, want: false},
		{args: []string{"exec", "--cmd", "ls"}, want: false},
		{args: []string{"tools", "sshx", "list"}, want: false},
		{args: []string{"tools", "netem", "set"}, want: false},
		{args: []string{"graph"}, want: false},
		{args: []string{"graph", "--dot"}, want: true},
		{args: []string{"graph", "--mermaid"}, want: true},
		{args: []string{"graph", "--offline"}, want: true},
		{args: []string{"graph", "--offline=false"}, want: false},
		{args: []string{"graph", "--drawio"}, want: false},
		{args: []string{"graph", "--offline", "--drawio"}, want: false},
		{args: []string{"graph", "--dot", "--name", "lab"}, want: false},
		{args: []string{"graph", "--dot", "--name", "lab", "-t", "lab.clab.yml"}, want: true},
		{args: []string{"graph"}, env: map[string]string{"CLAB_GRAPH_DOT": "true"}, want: true},
		{
			args: []string{"graph"},
			env:  map[string]string{"CLAB_GRAPH_OFFLINE": "false"},
			want: false,
		},
		{args: []string{"validate"}, want: true},
		{args: []string{"validate", "-t", "lab.clab.yml"}, want: true},
		{args: []string{"validate", "--name", "lab"}, want: false},
		{args: []string{"validate", "--name", "lab", "-t", "lab.clab.yml"}, want: true},
		{args: []string{"validate"}, env: map[string]string{"CLAB_NAME": "lab"}, want: false},
		{
			args: []string{"validate"},
			env:  map[string]string{"CLAB_NAME": "lab", "CLAB_TOPO": "lab.clab.yml"},
			want: true,
		},
		{args: []string{"tui"}, want: false},
		{args: []string{"tui", "--demo"}, want: true},
		{args: []string{"tui"}, env: map[string]string{"CLAB_TUI_DEMO": "true"}, want: true},
	}

	for _, tt := range tests {
		name := strings.Join(tt.args, " ")
		for key, value := range tt.env {
			name = key + "=" + value + " " + name
		}

		t.Run(name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			optionsInstance = nil
			t.Cleanup(func() { optionsInstance = nil })

			root, err := Entrypoint()
			if err != nil {
				t.Fatal(err)
			}
			root.InitDefaultHelpCmd()

			cobraCmd, flags, err := root.Find(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if err := cobraCmd.ParseFlags(flags); err != nil {
				t.Fatal(err)
			}
			// preRunFn applies the environment to the options before it checks for root
			updateOptionsFromViper(cobraCmd, GetOptions())

			if got := commandIsRuntimeIndependent(cobraCmd, GetOptions()); got != tt.want {
				t.Fatalf("commandIsRuntimeIndependent(%s) = %v, want %v", name, got, tt.want)
			}
		})
	}
}

func TestRuntimeIndependentCommandsRunWithoutRootWithPodman(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the root check passes as root, this test needs a non-root user")
	}
	t.Setenv("CLAB_VERSION_CHECK", "disable")
	t.Setenv("CLAB_RUNTIME", "podman")

	t.Run("tools cert ca create", func(t *testing.T) {
		optionsInstance = nil
		t.Cleanup(func() { optionsInstance = nil })

		root, err := Entrypoint()
		if err != nil {
			t.Fatal(err)
		}

		dir := t.TempDir()
		root.SetArgs([]string{"tools", "cert", "ca", "create", "--path", dir})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatal("tools cert ca create wrote no files")
		}
	})

	t.Run("shell completion", func(t *testing.T) {
		optionsInstance = nil
		t.Cleanup(func() { optionsInstance = nil })

		root, err := Entrypoint()
		if err != nil {
			t.Fatal(err)
		}

		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{cobra.ShellCompRequestCmd, "tools", ""})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(out.String(), "cert") {
			t.Fatalf("completion of tools doesn't offer cert: %q", out.String())
		}
	})
}
