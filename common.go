package fit

func JoinSvsPath(str ...string) string {
	return StringSpliceTag("/", str...)
}
