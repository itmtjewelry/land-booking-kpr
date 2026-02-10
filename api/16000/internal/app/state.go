package app

type State struct {
	StorageReady bool
	StorageDir   string
	LoadedFiles  []string
}
