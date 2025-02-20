package fit

import "github.com/pochard/commons/randstr"

type Random struct{}

func NewRandom() Random {
	return Random{}
}

// PureDigital  Generates a pure number of the specified length
func (r Random) PureDigital(len int) string {
	return randstr.RandomNumeric(len)
}

// LetterAndNumber Generate letters and numbers of specified length
func (r Random) LetterAndNumber(len int) string {
	return randstr.RandomAlphanumeric(len)
}

// Char Generate letters of specified length
func (r Random) Char(len int) string {
	return randstr.RandomAlphabetic(len)
}

// CharAndNumberAscii  Generate letters + numbers + other ASCII characters of the specified length
func (r Random) CharAndNumberAscii(len int) string {
	return randstr.RandomAscii(len)
}
