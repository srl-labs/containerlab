# Overview
This library provides the means to, without any cgo dependencies, adjust the regular linux filesystem ACLs (`system.posix_acl_access`) as well as the default ACLs (`system.posix_acl_default`).
It therefore is a golang native implementation of the getfacl / setfacl commands.

# Sample
The following code will try to load the actual ACL entries from the `filePath` referenced file.

If the `filePath` referenced object does not have an ACL attached, the regular file permissions are
loadded as ACL Entries.

Subsequentyl a new entry for the linux group with GID 5558 and a permission of 7 (rwx) is added.

Then the ACL is applied as an Access ACL to the `filePath` provided filesystem object.

```go
package main

import (
  log "github.com/sirupsen/logrus"
  "github.com/steiler/acls"
)

func main() {
    // Define the path to the file for which you want to get ACLs.
    filePath := "/tmp/foo"

    // init the ACL struct
    a := &acls.ACL{}
    // load (access) ACL entries from a given path object
    err := a.Load(filePath, acls.PosixACLAccess)
    if err != nil {
        log.Fatal(err)
    }
    // add a new entry referencing a group with GID 5558 granting permission rwx (7)
    err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_GROUP, 5558, 7))
    if err != nil {
        log.Fatal(err)
    }
    // print a visual representation of the ACL
    fmt.Println(a.String())

    // Apply the ACL as an access ACL to the given filesystem path object.
    err = a.Apply(filePath, acls.PosixACLAccess)
    if err != nil {
        log.Fatal(err)
    }
}
```

The output of the `fmt.Println(a.String())` looks like the following:

```
Version: 2
Entries:
Tag:   USER_OBJ ( 1), ID:       1000, Perm: rwx (7)
Tag:  GROUP_OBJ ( 4), ID:       1000, Perm: rwx (7)
Tag:      GROUP ( 8), ID:       5558, Perm: rwx (7)
Tag:       MASK (16), ID: 4294967295, Perm: rwx (7)
Tag:      OTHER (32), ID: 4294967295, Perm: r-x (5)
```

# Features
    - Add ACL Entry
    - Delete ACL Entry
    - Modify ACL Entry
    - Print ACL Entry
    - Read ACL entries from one file object, apply to another
    - Adjust default and access ACL
