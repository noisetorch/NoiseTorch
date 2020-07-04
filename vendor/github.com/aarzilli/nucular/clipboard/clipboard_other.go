// +build !windows,!linux,!darwin,!freebsd android

package clipboard

func Start() {
}

func Get() string {
	return ""
}

func GetPrimary() string {
	return ""
}

func Set(text string) {
}
