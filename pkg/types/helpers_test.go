package types

// generate32Bytes creates a [AddressSize]byte array from a string.
// It pads or truncates the input string to fit the fixed size.
func generate32Bytes(s string) [AddressSize]byte {
	var arr [AddressSize]byte
	copy(arr[:], []byte(s))
	return arr
}
