module github.com/srl-labs/containerlab

go 1.24.5

require (
	github.com/awalterschulze/gographviz v2.0.3+incompatible
	github.com/beevik/etree v1.6.0
	github.com/brunoga/deep v1.2.5
	github.com/charmbracelet/fang v0.4.4
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/charmbracelet/log v0.4.2
	github.com/containernetworking/plugins v1.9.0
	github.com/containers/podman/v5 v5.7.1
	github.com/digitalocean/go-openvswitch v0.0.0-20250625173537-a00eb8d2cfce
	github.com/distribution/reference v0.6.0
	github.com/docker/docker v28.5.2+incompatible
	github.com/docker/go-connections v0.6.0
	github.com/docker/go-units v0.5.0
	github.com/dustin/go-humanize v1.0.1
	github.com/florianl/go-tc v0.4.7
	github.com/go-xmlfmt/xmlfmt v1.1.3
	github.com/google/go-cmp v0.7.0
	github.com/google/nftables v0.3.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-version v1.8.0
	github.com/hellt/envsubst v0.2.0
	github.com/jedib0t/go-pretty/v6 v6.7.8
	github.com/joho/godotenv v1.5.1
	github.com/jsimonetti/rtnetlink v1.4.2
	github.com/klauspost/cpuid/v2 v2.3.0
	github.com/mackerelio/go-osstat v0.2.6
	github.com/minio/minio-go/v7 v7.0.98
	github.com/mitchellh/go-homedir v1.1.0
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/pmorjan/kmod v1.1.1
	github.com/scrapli/scrapligo v1.3.3
	github.com/scrapli/scrapligocfg v1.0.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
	github.com/steiler/acls v0.1.5
	github.com/stretchr/testify v1.11.1
	github.com/tklauser/numcpus v0.11.0
	github.com/vishvananda/netlink v1.3.1
	go.podman.io/common v0.66.1
	go.podman.io/image/v5 v5.38.0
	go.uber.org/mock v0.6.0
	golang.org/x/crypto v0.47.0
	golang.org/x/sys v0.41.0
	golang.org/x/term v0.39.0
	gopkg.in/yaml.v2 v2.4.0
	sigs.k8s.io/kind v0.31.0
)

require (
	al.essio.dev/pkg/shellescape v1.5.1 // indirect
	charm.land/lipgloss/v2 v2.0.0-beta.3.0.20251106193318-19329a3e8410 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/carlmontanari/difflibgo v0.0.0-20210718194309-31b9e131c298 // indirect
	github.com/charmbracelet/colorprofile v0.4.1 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20251106190538-99ea45596692 // indirect
	github.com/charmbracelet/x/ansi v0.11.5 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/exp/charmtone v0.0.0-20250603201427-c31516f43444 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.9.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.5.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/go-containerregistry v0.20.6 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20240620165639-de9c06129bec // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/mistifyio/go-zfs/v3 v3.1.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/mango v0.1.0 // indirect
	github.com/muesli/mango-cobra v1.2.0 // indirect
	github.com/muesli/mango-pflag v0.1.0 // indirect
	github.com/muesli/roff v0.1.0 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/opencontainers/cgroups v0.0.5 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.9 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.1 // indirect
	github.com/sigstore/fulcio v1.7.1 // indirect
	github.com/sigstore/protobuf-specs v0.4.1 // indirect
	github.com/sigstore/sigstore v1.9.5 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/smallstep/pkcs7 v0.1.1 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tchap/go-patricia/v2 v2.3.3 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/vbauerster/mpb/v8 v8.10.2 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.podman.io/storage v1.61.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20250103183323-7d7fa50e5329 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250414145226-207652e42e2e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250414145226-207652e42e2e // indirect
	tags.cncf.io/container-device-interface v1.0.1 // indirect
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.17.0 // indirect
	github.com/containers/buildah v1.42.2 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/containers/psgo v1.9.1-0.20250826150930-4ae76f200c86 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/cyphar/filepath-securejoin v0.5.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/docker/cli v28.5.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.4 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-git/go-git/v5 v5.16.4
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/godbus/dbus/v5 v5.1.1-0.20241109141217-c266b19b28e9 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/schema v1.4.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.4.0 // indirect
	github.com/klauspost/compress v1.18.3
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/mdlayher/netlink v1.8.0
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/term v0.5.2
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runc v1.3.4 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20250523060157-0ea5ed0382a2 // indirect
	github.com/opencontainers/selinux v1.13.1 // indirect
	github.com/proglottis/gpgme v0.1.5 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/sirikothe/gotextfsm v1.0.1-0.20200816110946-6aa2cfd355e4 // indirect
	github.com/spf13/pflag v1.0.10
	github.com/stefanberger/go-pkcs11uri v0.0.0-20230803200340-78284954bff6 // indirect
	github.com/sylabs/sif/v2 v2.22.0 // indirect
	github.com/ulikunitz/xz v0.5.15
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vishvananda/netns v0.0.5
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	golang.org/x/mod v0.32.0
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.34.3
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/weaveworks/libgitops v0.0.0-20200611103311-2c871bbbbf0c => github.com/networkop/libgitops v0.0.0-20221014201752-02ecf9790f58
