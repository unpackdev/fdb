package db

type Provider interface {
	Set(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Exists(key []byte) (bool, error)
	Delete(key []byte) error
	Close() error
	Destroy() error
}
