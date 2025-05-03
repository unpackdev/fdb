package share

import "strconv"

type Epoch int

var (
	FirstEpoch Epoch = 1
)

func (e Epoch) String() string {
	return strconv.Itoa(int(e))
}

func (e Epoch) Int() int {
	return int(e)
}

func (e Epoch) Cmp(e2 Epoch) bool {
	return e.Int() == e2.Int()
}

func (e Epoch) IsFirst() bool {
	return e.Cmp(FirstEpoch)
}

func (e Epoch) Previous() Epoch {
	return Epoch(e.Int() - 1)
}

func (e Epoch) Next() Epoch {
	return Epoch(e.Int() + 1)
}

func (e Epoch) Bytes() []byte {
	return []byte(e.String())
}
