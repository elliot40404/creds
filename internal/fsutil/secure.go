package fsutil

func SecureTree(dir string) error {
	if err := secureDir(dir); err != nil {
		return err
	}
	return secureTree(dir)
}
