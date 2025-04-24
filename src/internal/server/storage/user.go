package storage

type User struct {
	Name     string
	Password []byte

	ReadOnly bool
	Storage  *Storage
}
