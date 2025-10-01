[![GoDoc](https://godoc.org/github.com/pmorjan/kmod?status.svg)](https://godoc.org/github.com/pmorjan/kmod)

# kmod

A Go implementation of functions to load and unload Linux kernel modules.

Module dependencies are loaded / unloaded automatically as defined in
`<mod_dir>/modules.dep`.
Kmod uses the syscall finit_module(2) to load a kernel file into the kernel
and if that fails init_module(2). Compressed files are not supported
directly but users can provide a custom function to uncompress and load a module
file into the kernel. (**SetInitFunc**). This is to keep the number of external
dependencies low and also allows maximum flexibility.

See the simple examples below and [modprobe.go](cmd/modprobe/modprobe.go)
for an complete example.

```go
// Load uncompressed kernel module
package main

import (
    "log"

    "github.com/pmorjan/kmod"
)

func main() {
    k, err := kmod.New()
    if err != nil {
        log.Fatal(err)
    }
    if err := k.Load("brd", "rd_size=32768 rd_nr=16", 0); err != nil {
        log.Fatal(err)
    }
}
```

```go
// Load XZ compressed module
package main

import (
    "io/ioutil"
    "log"
    "os"

    "github.com/pmorjan/kmod"
    "github.com/ulikunitz/xz"
    "golang.org/x/sys/unix"
)

func main() {
    k, err := kmod.New(kmod.SetInitFunc(modInit))
    if err != nil {
        log.Fatal(err)
    }
    if err := k.Load("brd", "rd_size=32768 rd_nr=16", 0); err != nil {
        log.Fatal(err)
    }
}

func modInit(path, params string, flags int) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    rd, err := xz.NewReader(f)
    if err != nil {
        return err
    }
    buf, err := ioutil.ReadAll(rd)
    if err != nil {
        return err
    }
    return unix.InitModule(buf, params)
}
```
