package rbac

type FilePermission struct {
	View   bool
	Edit   bool
	Upload bool
	Delete bool
	Chmod  bool
	Chown  bool
}

var FileRolePermissions = map[string]FilePermission{
	"admin":    {View: true, Edit: true, Upload: true, Delete: true, Chmod: true, Chown: true},
	"operator": {View: true, Edit: true, Upload: true, Delete: false, Chmod: false, Chown: false},
	"viewer":   {View: true, Edit: false, Upload: false, Delete: false, Chmod: false, Chown: false},
}

func HasFilePermission(role, action string) bool {
	perms, ok := FileRolePermissions[role]
	if !ok {
		return false
	}

	switch action {
	case "view":
		return perms.View
	case "edit":
		return perms.Edit
	case "upload":
		return perms.Upload
	case "delete":
		return perms.Delete
	case "chmod":
		return perms.Chmod
	case "chown":
		return perms.Chown
	default:
		return false
	}
}
