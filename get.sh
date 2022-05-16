# The install script is based off of the Apache 2.0 script from Helm,
# https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3

: ${BINARY_NAME:="containerlab"}
: ${PROJECT_NAME:="containerlab"} # if project name does not match binary name
: ${USE_SUDO:="true"}
: ${USE_PKG:="true"} # default --use-pkg flag value. will use package installation by default unless the default is changed to false
: ${VERIFY_CHECKSUM:="false"}
: ${BIN_INSTALL_DIR:="/usr/bin"}
: ${REPO_NAME:="srl-labs/containerlab"}
: ${REPO_URL:="https://github.com/$REPO_NAME"}
: ${PROJECT_URL:="https://containerlab.dev"}

# detectArch discovers the architecture for this system.
detectArch() {
    ARCH=$(uname -m)
    case $ARCH in
    # armv5*) ARCH="armv5" ;;
    # armv6*) ARCH="armv6" ;;
    # armv7*) ARCH="arm" ;;
    aarch64) ARCH="arm64" ;;
    x86) ARCH="386" ;;
    x86_64) ARCH="amd64" ;;
    i686) ARCH="386" ;;
    i386) ARCH="386" ;;
    esac
}

# detectOS discovers the operating system for this system and its package format
detectOS() {
    OS=$(echo $(uname) | tr '[:upper:]' '[:lower:]')

    case "$OS" in
    # Minimalist GNU for Windows
    mingw*) OS='windows' ;;
    esac
    
    if [ -f /etc/os-release ]; then
        OS_ID="$(. /etc/os-release && echo "$ID")"
    fi
    case "${OS_ID}" in
        ubuntu|debian|raspbian)
            PKG_FORMAT="deb"
        ;;
        
        centos|rhel|sles)
            PKG_FORMAT="rpm"
        ;;
        
        *)
            if type "rpm" &>/dev/null; then
                PKG_FORMAT="rpm"
            elif type "dpkg" &>/dev/null; then
                PKG_FORMAT="deb"
            fi
        ;;
    esac
}

# runs the given command as root (detects if we are root already)
runAsRoot() {
    local CMD="$*"

    if [ $EUID -ne 0 -a $USE_SUDO = "true" ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

# verifySupported checks that the os/arch combination is supported
verifySupported() {
    local supported="linux-amd64\nlinux-386\nlinux-arm64"
    if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
        echo "No prebuilt binary for ${OS}-${ARCH}."
        echo "To build from source, go to ${REPO_URL}"
        exit 1
    fi

    if ! type "curl" &>/dev/null && ! type "wget" &>/dev/null; then
        echo "Either curl or wget is required"
        exit 1
    fi
}

# verifyOpenssl checks if openssl is installed to perform checksum operation
verifyOpenssl() {
    if [ $VERIFY_CHECKSUM == "true" ]; then
        if ! type "openssl" &>/dev/null; then
            echo "openssl is not found. It is used to verify checksum of the downloaded file."
            exit 1
        fi
    fi
}

# setDesiredVersion sets the desired version either to an explicit version provided by a user
# or to the latest release available on github releases
setDesiredVersion() {
    if [ "x$DESIRED_VERSION" == "x" ]; then
        # when desired version is not provided
        # get latest tag from the gh releases
        if type "curl" &>/dev/null; then
            local latest_release_url=$(curl -s https://api.github.com/repos/$REPO_NAME/releases/latest | sed '5q;d' | cut -d '"' -f 4)
            TAG=$(echo $latest_release_url | cut -d '"' -f 2 | awk -F "/" '{print $NF}')
            # tag with stripped `v` prefix
            TAG_WO_VER=$(echo "${TAG}" | cut -c 2-)
        elif type "wget" &>/dev/null; then
            # get latest release info and get 5th line out of the response to get the URL
            local latest_release_url=$(wget -q https://api.github.com/repos/$REPO_NAME/releases/latest -O- | sed '5q;d' | cut -d '"' -f 4)
            TAG=$(echo $latest_release_url | cut -d '"' -f 2 | awk -F "/" '{print $NF}')
            TAG_WO_VER=$(echo "${TAG}" | cut -c 2-)
        fi
    else
        TAG=$DESIRED_VERSION
        TAG_WO_VER=$(echo "${TAG}" | cut -c 2-)

        if type "curl" &>/dev/null; then
            if ! curl -s -o /dev/null --fail https://api.github.com/repos/$REPO_NAME/releases/tags/$DESIRED_VERSION; then
                echo "release $DESIRED_VERSION not found"
                exit 1
            fi
        elif type "wget" &>/dev/null; then
            if ! wget -q https://api.github.com/repos/$REPO_NAME/releases/tags/$DESIRED_VERSION; then
                echo "release $DESIRED_VERSION not found"
                exit 1
            fi
        fi
    fi
}

# checkInstalledVersion checks which version is installed and
# if it needs to be changed.
checkInstalledVersion() {
    if [[ -f "${BIN_INSTALL_DIR}/${BINARY_NAME}" ]]; then
        local version=$("${BIN_INSTALL_DIR}/${BINARY_NAME}" version | grep version | awk '{print $NF}')
        if [[ "v$version" == "$TAG" ]]; then
            echo "${BINARY_NAME} is already at its ${DESIRED_VERSION:-latest ($version)}" version
            return 0
        else
            echo "A newer ${BINARY_NAME} ${TAG_WO_VER} is available. Release notes: https://containerlab.dev/rn/${TAG_WO_VER}"
            echo "You are running containerlab $version version"
            UPGR_NEEDED="Y"
            # check if stdin is open (i.e. capable of getting users input)
            if [ -t 0 ]; then
                read -e -p "Proceed with upgrade? [Y/n]: " -i "Y" UPGR_NEEDED
            fi
            if [ "$UPGR_NEEDED" == "Y" ]; then
                return 1
            fi
            return 0
        fi
    else
        return 1
    fi
}

# createTempDir creates temporary directory where we downloaded files
createTempDir() {
    TMP_ROOT="$(mktemp -d)"
    TMP_BIN="$TMP_ROOT/$BINARY_NAME"
}

# downloadFile downloads the latest binary archive, the checksum file and performs the sum check
downloadFile() {
    EXT="tar.gz" # download file extension
    if [ $USE_PKG == "true" ]; then
        EXT=$PKG_FORMAT
    fi
    ARCHIVE="${PROJECT_NAME}_${TAG_WO_VER}_${OS}_${ARCH}.${EXT}"
    DOWNLOAD_URL="${REPO_URL}/releases/download/${TAG}/${ARCHIVE}"
    CHECKSUM_URL="${REPO_URL}/releases/download/${TAG}/checksums.txt"
    TMP_FILE="$TMP_ROOT/$ARCHIVE"
    SUM_FILE="$TMP_ROOT/checksums.txt"
    echo "Downloading $DOWNLOAD_URL"
    if type "curl" &>/dev/null; then
        curl -SsL "$CHECKSUM_URL" -o "$SUM_FILE"
        curl -SsL "$DOWNLOAD_URL" -o "$TMP_FILE"
    elif type "wget" &>/dev/null; then
        wget -q -O "$SUM_FILE" "$CHECKSUM_URL"
        wget -q -O "$TMP_FILE" "$DOWNLOAD_URL"
    fi

    # verify downloaded file
    if [ $VERIFY_CHECKSUM == "true" ]; then
        local sum=$(openssl sha1 -sha256 ${TMP_FILE} | awk '{print $2}')
        local expected_sum=$(cat ${SUM_FILE} | grep -i $ARCHIVE | awk '{print $1}')
        if [ "$sum" != "$expected_sum" ]; then
            echo "SHA sum of ${TMP_FILE} does not match. Aborting."
            exit 1
        fi
        echo "Checksum verified"
    fi
}

# installFile verifies the SHA256 for the file, then unpacks and
# installs it. By default, the installation is done from .tar.gz archive, that can be overriden with --use-pkg flag
installFile() {
    tar xf "$TMP_FILE" -C "$TMP_ROOT"
    echo "Preparing to install $BINARY_NAME ${TAG_WO_VER} into ${BIN_INSTALL_DIR}"
    runAsRoot cp "$TMP_ROOT/$BINARY_NAME" "$BIN_INSTALL_DIR/$BINARY_NAME"
    echo "$BINARY_NAME installed into $BIN_INSTALL_DIR/$BINARY_NAME"
}

# installPkg installs the downloaded version of a package in a deb or rpm format
installPkg() {
    echo "Preparing to install $BINARY_NAME ${TAG_WO_VER} from package"
    if [ $PKG_FORMAT == "deb" ]; then
        runAsRoot dpkg -i $TMP_FILE
    elif [ $PKG_FORMAT == "rpm" ]; then
        runAsRoot rpm -U $TMP_FILE
    fi
}

# fail_trap is executed if an error occurs.
fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        if [[ -n "$INPUT_ARGUMENTS" ]]; then
            echo "Failed to install $BINARY_NAME with the arguments provided: $INPUT_ARGUMENTS"
            help
        else
            echo "Failed to install $BINARY_NAME"
        fi
        echo -e "\tFor support, go to $REPO_URL/issues"
    fi
    cleanup
    exit $result
}

# testVersion tests the installed client to make sure it is working.
testVersion() {
    set +e
    $BIN_INSTALL_DIR/$BINARY_NAME version
    if [ "$?" = "1" ]; then
        echo "$BINARY_NAME not found. Is $BIN_INSTALL_DIR in your "'$PATH?'
        exit 1
    fi
    set -e
}

# help provides possible cli installation arguments
help() {
    echo "Accepted cli arguments are:"
    echo -e "\t[--help|-h ] ->> prints this help"
    echo -e "\t[--version|-v <desired_version>] . When not defined it fetches the latest release from GitHub"
    echo -e "\te.g. --version v0.1.1"
    echo -e "\t[--use-pkg]  ->> install from deb/rpm packages"
    echo -e "\t[--no-sudo]  ->> install without sudo"
    echo -e "\t[--verify-checksum]  ->> verify checksum of the downloaded file"
}

# removes temporary directory used to download artefacts
cleanup() {
    if [[ -d "${TMP_ROOT:-}" ]]; then
        rm -rf "$TMP_ROOT"
    fi
}

# Execution

# Stop execution on any error
trap "fail_trap" EXIT
set -e

# Parsing input arguments (if any)
export INPUT_ARGUMENTS="${@}"
set -u
while [[ $# -gt 0 ]]; do
    case $1 in
    '--version' | -v)
        shift
        if [[ $# -ne 0 ]]; then
            export DESIRED_VERSION="v${1}"
        else
            echo -e "Please provide the desired version. e.g. --version 0.1.1"
            exit 0
        fi
        ;;
    '--no-sudo')
        USE_SUDO="false"
        ;;
    '--verify-checksum')
        VERIFY_CHECKSUM="true"
        ;;
    '--use-pkg')
        USE_PKG="true"
        ;;
    '--help' | -h)
        help
        exit 0
        ;;
    *)
        exit 1
        ;;
    esac
    shift
done
set +u

detectArch
detectOS
verifySupported
setDesiredVersion
if ! checkInstalledVersion; then
    createTempDir
    verifyOpenssl
    downloadFile
    if [ $USE_PKG == "true" ]; then
        installPkg
    else
        installFile
    fi
    testVersion
    cleanup
fi
