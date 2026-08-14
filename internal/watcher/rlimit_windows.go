package watcher

// processFDLimit is not meaningful on Windows: ReadDirectoryChangesW watches
// directories with a single handle each and does not open per-file handles.
func processFDLimit() uint64 {
	return 0
}
