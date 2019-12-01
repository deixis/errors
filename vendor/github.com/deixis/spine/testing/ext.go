package testing

// DidPanic returns whether the given function panicked
func DidPanic(f func()) (bool, interface{}) {
	p := false
	var err interface{}

	func() {
		defer func() {
			if err = recover(); err != nil {
				p = true
			}
		}()

		f() // call the target function
	}()

	return p, err
}
