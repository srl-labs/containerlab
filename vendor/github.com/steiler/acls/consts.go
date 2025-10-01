package acls

type ACLAttr string

const (
	PosixACLAccess  ACLAttr = "system.posix_acl_access"
	PosixACLDefault ACLAttr = "system.posix_acl_default"
)

type Tag uint16

const (
	// Undefined ACL type.
	TAG_ACL_UNDEFINED_FIELD = 0x0
	// Discretionary access rights for
	//processes whose effective user ID
	//matches the user ID of the file's owner.
	TAG_ACL_USER_OBJ = 0x1
	// Discretionary access rights for
	// processes whose effective user ID
	// matches the ACL entry qualifier.
	TAG_ACL_USER = 0x2
	// Discretionary access rights for
	// processes whose effective groupID or
	// any supplemental groups match the group
	// ID of the file's owner.
	TAG_ACL_GROUP_OBJ = 0x4
	// Discretionary access rights for
	// processes whose effective group ID or
	// any supplemental groups match the ACL
	// entry qualifier.
	TAG_ACL_GROUP = 0x8
	// The maximum discretionary access rights
	// that can be granted to a process in the
	// file group class. This is only valid
	// for POSIX.1e ACLs.
	TAG_ACL_MASK = 0x10
	// Discretionary access rights for
	// processes not covered by any other ACL
	// entry. This is only valid for POSIX.1e
	// ACLs.
	TAG_ACL_OTHER = 0x20
	// Same as ACL_OTHER.
	TAG_ACL_OTHER_OBJ = TAG_ACL_OTHER
	// Discretionary access rights for all
	// users. This is only valid for NFSv4
	// ACLs.
	TAG_ACL_EVERYONE = 0x40
)

func Tag2String(i Tag) string {
	result := "undefined"
	switch i {
	case 0x0:
		// undefiend
	case 0x1:
		result = "USER_OBJ"
	case 0x2:
		result = "USER"
	case 0x4:
		result = "GROUP_OBJ"
	case 0x8:
		result = "GROUP"
	case 0x10:
		result = "MASK"
	case 0x20, 0x30:
		result = "OTHER"
	case 0x40:
		result = "EVERYONE"
	}
	return result
}
