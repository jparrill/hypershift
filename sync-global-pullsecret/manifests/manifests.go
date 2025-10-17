package manifests

// CRIOGlobalAuthFileConfig returns the CRIO configuration content for setting global_auth_file.
// This creates a drop-in configuration file for /etc/crio/crio.conf.d/ that points CRIO
// to use a custom authentication file for all registry operations.
func CRIOGlobalAuthFileConfig(authFilePath string) string {
	return `[crio]
log_level = "debug"
[crio.image]
global_auth_file = "` + authFilePath + `"
pause_image_auth_file = "` + authFilePath + `"
`
}
