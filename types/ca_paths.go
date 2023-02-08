package types

type CaPaths interface {
	NodeCertKeyAbsFilename(identifier string) string
	NodeCertAbsFilename(identifier string) string
	NodeCertCSRAbsFilename(identifier string) string
	CANodeDir(string) string
	RootCaIdentifier() string
}

type CaPathsImpl struct {
}
