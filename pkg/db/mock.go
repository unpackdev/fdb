package db

// MockDB implements db.Provider for testing
type MockDB struct {
	data map[string][]byte
}

func NewMockDB() *MockDB {
	return &MockDB{
		data: make(map[string][]byte),
	}
}

func (m *MockDB) Set(key, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *MockDB) Get(key []byte) ([]byte, error) {
	value, ok := m.data[string(key)]
	if !ok {
		return nil, nil
	}
	return value, nil
}

func (m *MockDB) Exists(key []byte) (bool, error) {
	_, ok := m.data[string(key)]
	return ok, nil
}

func (m *MockDB) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *MockDB) Close() error {
	return nil
}

func (m *MockDB) Destroy() error {
	m.data = make(map[string][]byte)
	return nil
}
