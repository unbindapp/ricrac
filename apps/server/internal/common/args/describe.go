package args

// DescribeDecodeError says what is wrong with JSON that didn't decode, as argument errors do.
func DescribeDecodeError(err error) string {
	return describeDecodeError(err)
}
