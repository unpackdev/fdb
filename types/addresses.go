package types

var (
	ZeroAddress = Address{}

	// StakingAddress is the address used for staking transactions.
	StakingAddress = Address{19: 0x01} // Set the last byte to 0x01; others default to 0x00
)
