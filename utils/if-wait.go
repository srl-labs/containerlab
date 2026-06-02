package utils

import _ "embed"

// IfWaitScript is used in ENTRYPOINT/CMD of the nodes that need to ensure that all
// of the clab links/interfaces are available in the container before calling the main process.
//
//go:embed if-wait.sh
var IfWaitScript string
