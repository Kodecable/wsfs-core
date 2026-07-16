//go:build unix && !linux

package client

func fuseConnectionID(string) (*uint64, error) {
	return nil, nil
}
